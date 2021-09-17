package remind

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/diamondburned/arikawa/v2/api"
	"github.com/diamondburned/arikawa/v2/discord"
	"github.com/diamondburned/arikawa/v2/session"
	"github.com/google/uuid"
	date "github.com/joyt/godate"
	"github.com/sirupsen/logrus"
)

// Reminder represents a Reminder process object
type Reminder struct {
	RemindsFilePath string
	DiscordSession  *session.Session
	tasks           []task
}

type task struct {
	TimeStr   time.Time
	Message   string
	ChannelID discord.ChannelID
	ID        string
}

// CommandData returns request
func CommandData() api.CreateCommandData {
	return api.CreateCommandData{
		Name:        "remind",
		Description: "send the given message in the given channel at the specified time",
		Options: []discord.CommandOption{
			{
				Name:        "when",
				Type:        discord.StringOption,
				Description: "in how long to send the message",
				Required:    true,
			},
			{
				Name:        "message",
				Type:        discord.StringOption,
				Description: "what message to send",
				Required:    true,
			},
			{
				Name: "channel",
				Type: discord.ChannelOption,
				Description: "specify what channel to send the message, if none is given this " +
					"channel will be used",
				Required: false,
			},
		},
	}
}

// CreateRemindStrict creates a remind-task
func (r *Reminder) CreateRemindStrict(
	timeStr string, message string, channelID discord.ChannelID,
) (*api.InteractionResponseData, error) {
	task, err := r.createRemind(timeStr, message, channelID)
	if err != nil {
		return nil, err
	}

	r.tasks = append(r.tasks, task)
	go r.saveTasks()

	return &api.InteractionResponseData{
		Content: "👍",
	}, nil
}

func (r *Reminder) createRemind(
	timeStr string, message string, channelID discord.ChannelID,
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

		err = nil
	}

	t = task{
		TimeStr:   timestamp,
		Message:   message,
		ChannelID: channelID,
		ID:        uuid.New().String(),
	}

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
		logrus.Error(err)
	}
	err = ioutil.WriteFile(r.RemindsFilePath, dat, 0x600)
	if err != nil {
		logrus.Error(err)
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
		logrus.Infof("task with id %s not found", id)
		return
	}
	r.tasks = r.tasks[:i+copy(r.tasks[i:], r.tasks[i+1:])]
	r.saveTasks()
}

func (r *Reminder) queueRemind(t task) {
	logrus.Infof("created reminder with datetime %q and message %q", t.TimeStr, t.Message)
	go func(tsk task) {
		if tsk.TimeStr.After(time.Now()) { // check if remind-time is in the future
			// if so wait
			timer := time.NewTimer(tsk.TimeStr.Sub(time.Now()))
			<-timer.C
		}
		r.DiscordSession.SendText(tsk.ChannelID, tsk.Message)
		r.deleteTask(tsk.ID) // delete handled task
	}(t)
}

// Start reads the reminds-file and queues all persisted reminds
func (r *Reminder) Start() (err error) {
	_, err = os.Stat(r.RemindsFilePath)
	if os.IsNotExist(err) {
		logrus.Infof("file %s not found, starting anew!", r.RemindsFilePath)
		// cancel reading reminds file. It did not exists
		return
	}

	dat, err := ioutil.ReadFile(r.RemindsFilePath)
	if err != nil {
		return
	}

	err = json.Unmarshal(dat, &r.tasks)
	if err != nil {
		return
	}

	for _, task := range r.tasks {
		r.queueRemind(task)
	}

	return
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
