package remind

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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

	if err != nil {
		timeSplits := strings.Split(timeStr, "+")

		totalDuration := time.Duration(0)

		for _, ts := range timeSplits {
			numberStr, timeDenot := parseTimeStr(ts)
			var i int
			i, err = strconv.Atoi(numberStr)
			if err != nil {
				err = fmt.Errorf("integer in %s is not a valid integer", ts)
				return
			}

			duratn := getDuration(timeDenot)
			if duratn == -1 {
				err = fmt.Errorf("duration denotation in %s is invalid", ts)
				return
			}

			totalDuration += duratn * time.Duration(i)
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

func parseTimeStr(s string) (numbers string, letters string) {
	var l, n []rune
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			l = append(l, r)
		case r >= 'a' && r <= 'z':
			l = append(l, r)
		case r >= '0' && r <= '9':
			n = append(n, r)
		}
	}
	return string(n), string(l)
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
	dat, err := ioutil.ReadFile("reminds.json")
	if err != nil {
		log.Fatal(err)
	}

	json.Unmarshal(dat, &r.tasks)

	for _, task := range r.tasks {
		r.queueRemind(task)
	}
}

func getDuration(denotation string) time.Duration {
	switch denotation {
	case "yr", "yrs", "year", "years", "y":
		return time.Minute * 525600

	case "month", "months", "mnth", "mnd", "måned", "måneder":
		return time.Minute * 43800

	case "week", "weeks", "wk", "uke":
		return time.Minute * 10080

	case "day", "days", "dag", "dager":
		return time.Minute * 1440

	case "hour", "hr", "h", "time", "timer":
		return time.Minute * 60

	case "minute", "minutes", "minutt", "minutter", "min", "mins", "m", "":
		return time.Minute

	case "seconds", "s", "sekund", "sekunder":
		return time.Second

	case "ms":
		return time.Millisecond

	case "ns":
		return time.Nanosecond
	}

	return -1
}
