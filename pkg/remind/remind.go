package remind

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	date "github.com/joyt/godate"
	log "github.com/sirupsen/logrus"
)

// Reminder represents a Reminder process object
type Reminder struct {
	DiscordSession *discordgo.Session
	tasks          []task
}

type task struct {
	TimeStr   time.Time
	Message   string
	ChannelID string
	ID        string
}

// CreateRemindStrict creates a remind-task
func (r *Reminder) CreateRemindStrict(
	timeStr string, message string, channelID string,
) error {
	task, err := r.createRemind(timeStr, message, channelID)
	if err != nil {
		return err
	}

	r.tasks = append(r.tasks, task)
	go r.saveTasks()
	return nil
}

func (r *Reminder) createRemind(
	timeStr string, message string, channelID string,
) (t task, err error) {
	var timestamp time.Time
	timestamp, err = date.ParseInLocation(strings.Replace(timeStr, "+", " ", -1), time.Local)

	// if we failed parsing the time-string as a timedate, we attempt to
	if err != nil {
		timeSplits := strings.Split(timeStr, "+")

		totalDuration := time.Duration(0)

		for _, ts := range timeSplits {
			duratn, parseErr := parseStringToDuration(ts)
			if parseErr != nil {
				err = fmt.Errorf("error occurred parsing time-string: %v", parseErr)
				return
			}

			totalDuration += duratn
		}

		timestamp = time.Now().Add(totalDuration)
	}

	id, err := uuid.NewUUID()
	if err != nil {
		return
	}

	t = task{timestamp, message, channelID, id.String()}

	r.queueRemind(t)
	return
}

// parseStringToDuration takes a string containing a number with denotation
// (e. g. 14m, 17y, 1month or 8hr) and returns it as a time.Duration
func parseStringToDuration(s string) (time.Duration, error) {
	var l, n []rune
	// iterate over string and populate l with letters, n with numbers
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z':
			l = append(l, r)
		case r >= '0' && r <= '9':
			n = append(n, r)
		}
	}

	num, err := strconv.Atoi(string(n))
	if err != nil {
		return 0, err
	}

	if num < 1 {
		return 0, fmt.Errorf("number was smaller than 1")
	}

	denot := denotationAsDuration(string(l))
	if denot == -1 {
		return 0, fmt.Errorf("error occurred parsing denotation %q", string(l))
	}

	return denot * time.Duration(num), nil
}

func (r *Reminder) saveTasks() {
	dat, err := json.Marshal(r.tasks)
	if err != nil {
		log.Error(err)
	}
	err = ioutil.WriteFile("reminds.json", dat, 0x600)
	if err != nil {
		log.Error(err)
	}
}

func (r *Reminder) deleteTask(id string) {
	i := -1
	for index, t := range r.tasks {
		if t.ID == id {
			i = index
			break
		}
	}
	if i == -1 {
		log.Infof("task with id %s not found", id)
		return
	}
	r.tasks = r.tasks[:i+copy(r.tasks[i:], r.tasks[i+1:])]
	r.saveTasks()
}

func (r *Reminder) queueRemind(t task) {
	log.Infof("created reminder with datetime %q and message %q", t.TimeStr, t.Message)
	go func(tsk task) {
		if tsk.TimeStr.After(time.Now()) { // check if remind-time is in the future
			// if so wait
			timer := time.NewTimer(tsk.TimeStr.Sub(time.Now()))
			<-timer.C
		}
		r.DiscordSession.ChannelMessageSend(tsk.ChannelID, tsk.Message)
		r.deleteTask(tsk.ID) // delete handled task
	}(t)
}

// Start reads the reminds-file and queues all persisted reminds
func (r *Reminder) Start() {
	if _, err := os.Stat("reminds.json"); os.IsNotExist(err) {
		log.Info("file reminds.json not found, starting anew!")
		return
	}

	dat, err := ioutil.ReadFile("reminds.json")
	if err != nil {
		log.Fatal(err)
	}

	json.Unmarshal(dat, &r.tasks)

	for _, task := range r.tasks {
		r.queueRemind(task)
	}
}

func denotationAsDuration(denotation string) time.Duration {
	switch denotation {
	case "yr", "yrs", "year", "years", "y":
		return time.Minute * 525600

	case "month", "months", "mnth", "mnd", "måned", "måneder":
		return time.Minute * 43800

	case "w", "week", "weeks", "wk", "uke", "uker":
		return time.Minute * 10080

	case "d", "day", "days", "dag", "dager":
		return time.Minute * 1440

	case "h", "hour", "hr", "time", "timer":
		return time.Hour

	case "m", "minute", "minutes", "minutt", "minutter", "min", "mins", "":
		return time.Minute

	case "s", "seconds", "sekund", "sekunder":
		return time.Second

	case "ms":
		return time.Millisecond

	case "ns":
		return time.Nanosecond
	}

	return -1
}
