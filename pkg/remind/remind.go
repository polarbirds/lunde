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

func (r *Reminder) CreateRemind(timeStr string, message string, channelID string) error {
	task, err := r.createRemind(timeStr, message, channelID)
	if err != nil {
		return err
	}

	r.tasks = append(r.tasks, task)
	go r.saveTasks()
	return nil
}

func (r *Reminder) createRemind(timeStr string, message string, channelID string) (t task, err error) {
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
	case "yr":
		fallthrough
	case "yrs":
		fallthrough
	case "year":
		fallthrough
	case "years":
		fallthrough
	case "y":
		return time.Minute * 525600

	case "month":
		fallthrough
	case "months":
		fallthrough
	case "mnth":
		fallthrough
	case "mnd":
		fallthrough
	case "måned":
		fallthrough
	case "måneder":
		return time.Minute * 43800

	case "week":
		fallthrough
	case "weeks":
		fallthrough
	case "wk":
		fallthrough
	case "uke":
		return time.Minute * 10080

	case "day":
		fallthrough
	case "days":
		fallthrough
	case "dag":
		fallthrough
	case "dager":
		return time.Minute * 1440

	case "hour":
		fallthrough
	case "hr":
		fallthrough
	case "h":
		fallthrough
	case "time":
		fallthrough
	case "timer":
		return time.Minute * 60

	case "minute":
		fallthrough
	case "minutes":
		fallthrough
	case "minutt":
		fallthrough
	case "minutter":
		fallthrough
	case "min":
		fallthrough
	case "mins":
		fallthrough
	case "m":
		fallthrough
	case "":
		return time.Minute

	case "seconds":
		fallthrough
	case "s":
		fallthrough
	case "sekund":
		fallthrough
	case "sekunder":
		return time.Second

	case "ms":
		return time.Millisecond

	case "ns":
		return time.Nanosecond
	}

	return -1
}
