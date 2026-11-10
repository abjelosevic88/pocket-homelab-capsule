package server

import (
	"net/http"
	"time"
)

// template is a kind of capsule to start from: what it is for, when it
// usually opens, and ideas of what to put in.
type template struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Emoji     string     `json:"emoji"`
	About     string     `json:"about"`
	Rule      string     `json:"open_rule"` // date | birthday | anniversary | newyear
	Years     int        `json:"years"`     // opens this many years later (or on this birthday / anniversary)
	Together  bool       `json:"together"`  // made to be filled with others
	Prompts   []string   `json:"prompts"`   // what to write about
	Questions []string   `json:"questions"` // for whoever opens it
	Guesses   []string   `json:"predictions"`
	Lists     []tmplList `json:"lists"`
}

type tmplList struct {
	Title  string   `json:"title"`
	Labels []string `json:"labels"`
}

var listNow = tmplList{"Right now", []string{"Favourite song", "Favourite food", "Best friend", "What I do most evenings", "Something I want to learn", "What worries me", "What makes me happy"}}
var listPrices = tmplList{"What things cost", []string{"A loaf of bread", "A litre of milk", "A coffee in town", "A litre of petrol", "A cinema ticket", "Rent or mortgage a month", "A new phone"}}

var templates = []template{
	{ID: "self", Name: "Letter to future me", Emoji: "✉️", About: "A letter, a few questions and some guesses, for you in a year (or ten).", Rule: "date", Years: 1,
		Prompts:   []string{"What does a normal day look like for you right now?", "What are you proud of this year?", "What are you worried about, and what do you hope happens?", "Who matters most to you right now, and why?", "What advice would you give the person reading this?"},
		Questions: []string{"Do you still live in the same place?", "Did you learn the thing you wanted to learn?", "Are you still friends with the people you named?", "What do you wish you had known when you wrote this?"},
		Guesses:   []string{"I will have changed jobs", "I will have visited a new country", "I will still be listening to the same music"},
		Lists:     []tmplList{listNow, listPrices}},
	{ID: "newyear", Name: "New Year's capsule", Emoji: "🎆", About: "Your hopes for the year, opened next New Year's Eve.", Rule: "newyear", Years: 1,
		Prompts:   []string{"How did last year go, in a few lines?", "Three things you want from this year", "One habit to start and one to stop", "Where do you hope to be when you open this?"},
		Questions: []string{"Which of the three things happened?", "What surprised you most this year?"},
		Guesses:   []string{"I will keep my resolution past March", "The biggest change this year will be at home", "I will read more than 10 books"},
		Lists:     []tmplList{{"Goals for the year", []string{"Health", "Work", "Money", "People", "Fun"}}, listNow}},
	{ID: "child", Name: "For my child", Emoji: "🧸", About: "Letters, photos and voices from their first years, opened on their 18th birthday.", Rule: "birthday", Years: 18, Together: true,
		Prompts:   []string{"The day you were born", "What you are like right now: your laugh, your words, what you love", "What we did together this year", "What we hope for you", "What the world is like right now"},
		Questions: []string{"Do you remember any of this?", "What would you like to ask us now?"},
		Guesses:   []string{"You will love music", "You will be taller than your dad", "You will still have the blue teddy"},
		Lists:     []tmplList{{"Your favourites", []string{"Toy", "Food", "Song", "Book", "Word you say all the time", "Best friend"}}, listPrices}},
	{ID: "family", Name: "Family capsule", Emoji: "🏡", About: "Everyone adds something; nobody sees the rest until it opens together.", Rule: "date", Years: 5, Together: true,
		Prompts:   []string{"What our family is like right now", "A favourite memory from this year", "Something I never said out loud", "What I hope for all of us"},
		Questions: []string{"Who changed the most?", "Which guess was the funniest?"},
		Guesses:   []string{"We will still live in this house", "Someone will have a new pet", "We will have been on a big trip together"},
		Lists:     []tmplList{{"Who we are", []string{"Who cooks best", "Who is always late", "Who tells the best jokes", "Where we go on holiday"}}, listPrices}},
	{ID: "couple", Name: "Our capsule", Emoji: "💞", About: "For the two of you, opened on an anniversary.", Rule: "anniversary", Years: 10, Together: true,
		Prompts:   []string{"How we met, as I remember it", "What I love about you right now", "Our life together today", "What I hope we do in the next years"},
		Questions: []string{"Do we still have the same Sunday?", "What would you tell us back then?"},
		Guesses:   []string{"We will have moved", "We will still fight about the dishes"},
		Lists:     []tmplList{{"Us right now", []string{"Our song", "Our place", "Our favourite meal", "Our running joke", "Where we want to travel"}}}},
	{ID: "birthday", Name: "Birthday letter", Emoji: "🎂", About: "Written on your birthday, opened on the next one.", Rule: "birthday", Years: 0,
		Prompts:   []string{"How this year of your life went", "What you want from the next one", "Who you spent today with"},
		Questions: []string{"Did the year go the way you hoped?"},
		Guesses:   []string{"I will celebrate somewhere else next year"},
		Lists:     []tmplList{listNow}},
	{ID: "team", Name: "Team capsule", Emoji: "🚀", About: "What the team expects, opened when the project ends (or a year from now).", Rule: "date", Years: 1, Together: true,
		Prompts:   []string{"What we are building and why", "What I am most unsure about", "What I hope we will be proud of"},
		Questions: []string{"What took the longest?", "What would we do differently?"},
		Guesses:   []string{"We ship by the end of the year", "We will have more than 1000 users", "The hardest part will be the start"},
		Lists:     []tmplList{{"The team today", []string{"How many we are", "Tools we use", "Biggest risk", "Best decision so far"}}}},
	{ID: "graduation", Name: "Graduation capsule", Emoji: "🎓", About: "Where you are when school ends, opened ten years later.", Rule: "date", Years: 10, Together: true,
		Prompts:   []string{"What school was like", "Where you think you will be in ten years", "Friends you want to keep"},
		Questions: []string{"Are you doing what you thought you would?", "Who are you still in touch with?"},
		Guesses:   []string{"I will live in another city", "I will have a job I can't imagine yet"},
		Lists:     []tmplList{listNow, listPrices}},
	{ID: "blank", Name: "Blank capsule", Emoji: "⏳", About: "Start empty and fill it your way.", Rule: "date", Years: 1},
}

func templateByID(id string) template {
	for _, t := range templates {
		if t.ID == id {
			return t
		}
	}
	return templates[len(templates)-1]
}

// defaultOpens is when a template's capsule opens if nobody picks a day,
// at the person's opening hour.
func defaultOpens(t template, hour int, now time.Time) time.Time {
	y := t.Years
	if y <= 0 {
		y = 1
	}
	d := time.Date(now.Year()+y, now.Month(), now.Day(), hour, 0, 0, 0, time.Local)
	if t.Rule == "newyear" {
		d = time.Date(now.Year()+1, time.January, 1, 0, 0, 0, 0, time.Local)
	}
	return d
}

func (s *Server) handleTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"templates": templates})
}
