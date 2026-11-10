package store

import (
	"encoding/json"
	"time"
)

// User is an account on this box.
type User struct {
	ID           int64  `json:"id"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"` // admin | member
	CreatedAt    string `json:"created_at"`
}

// APIToken is a personal access token (hash stored, plaintext shown once).
type APIToken struct {
	ID         int64   `json:"id"`
	UserID     int64   `json:"user_id"`
	Name       string  `json:"name"`
	Prefix     string  `json:"prefix"`
	LastUsedAt *string `json:"last_used_at"`
	CreatedAt  string  `json:"created_at"`
}

// Capsule states.
const (
	StateFilling = "filling"
	StateSealed  = "sealed"
	StateOpened  = "opened"
)

// Capsule is a time capsule: what is inside, when it opens, who it is for.
type Capsule struct {
	ID          int64           `json:"id"`
	UserID      int64           `json:"owner_id"`
	OwnerName   string          `json:"owner_name"`
	Title       string          `json:"title"`
	About       string          `json:"about"`
	Template    string          `json:"template"`
	Emoji       string          `json:"emoji"`
	Color       string          `json:"color"`
	State       string          `json:"state"`
	OpensAt     string          `json:"opens_at"`
	OpenRule    string          `json:"open_rule"`
	ForName     string          `json:"for_name"`
	ForDate     string          `json:"for_date"`
	ForAge      int             `json:"for_age"`
	Surprise    bool            `json:"surprise"`
	Slot        bool            `json:"slot"`
	Deliver     string          `json:"deliver"`
	RemindDays  int             `json:"remind_days"`
	Sections    []string        `json:"sections"`
	Snapshot    json.RawMessage `json:"snapshot,omitempty"`
	SealedAt    *string         `json:"sealed_at"`
	OpenedAt    *string         `json:"opened_at"`
	OpenedBy    *int64          `json:"opened_by"`
	Early       bool            `json:"early"`
	EarlyReason string          `json:"early_reason"`
	ReadySent   bool            `json:"-"`
	RemindSent  bool            `json:"-"`
	DeliveredAt *string         `json:"delivered_at"`
	Reflection  string          `json:"reflection"`
	ThenNow     string          `json:"then_now"`
	ReplyTo     *int64          `json:"reply_to"`
	GiveKey     string          `json:"-"`
	GiveOpen    bool            `json:"give_open"`
	CalendarURL string          `json:"calendar_url"`
	Source      string          `json:"source"`
	ExtID       string          `json:"-"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`

	// Filled in for lists.
	Items   int `json:"items"`
	Members int `json:"members_n"`
}

// Opens is when the capsule may be opened.
func (c *Capsule) Opens() time.Time {
	t, _ := time.Parse(time.RFC3339Nano, c.OpensAt)
	return t
}

// Ready says the day has come for a sealed capsule.
func (c *Capsule) Ready(now time.Time) bool {
	return c.State == StateSealed && !now.Before(c.Opens())
}

// Member is a person on this box in a capsule.
type Member struct {
	CapsuleID int64  `json:"capsule_id"`
	UserID    int64  `json:"user_id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Role      string `json:"role"` // contributor | viewer
	AddedAt   string `json:"added_at"`
}

// Recipient is someone a capsule is for, reached by email and their own link.
type Recipient struct {
	ID        int64   `json:"id"`
	UserID    int64   `json:"-"`
	CapsuleID int64   `json:"capsule_id"`
	Name      string  `json:"name"`
	Email     string  `json:"email"`
	Key       string  `json:"-"`
	SentAt    *string `json:"sent_at"`
	ViewedAt  *string `json:"viewed_at"`
	CreatedAt string  `json:"created_at"`
}

// Entry is one line of a list item ("Favourite song: …").
type Entry struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Item is one thing inside a capsule.
type Item struct {
	ID         int64   `json:"id"`
	CapsuleID  int64   `json:"capsule_id"`
	UserID     *int64  `json:"user_id"`
	AuthorName string  `json:"author_name"`
	Kind       string  `json:"kind"`
	Title      string  `json:"title"`
	Body       string  `json:"body"`
	UploadID   *int64  `json:"upload_id"`
	Confidence int     `json:"confidence"`
	Outcome    string  `json:"outcome"`
	Answer     string  `json:"answer"`
	Entries    []Entry `json:"entries"`
	URL        string  `json:"url"`
	FromApp    string  `json:"from_app"`
	FromURL    string  `json:"from_url"`
	AfterSeal  bool    `json:"after_seal"`
	Guest      bool    `json:"guest"`
	Position   int     `json:"position"`
	Source     string  `json:"source"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`

	// Filled in from the upload.
	Mime  string `json:"mime,omitempty"`
	Name  string `json:"file_name,omitempty"`
	Bytes int    `json:"bytes,omitempty"`
}

// ItemKinds are the things a capsule can hold.
var ItemKinds = []string{"letter", "photo", "audio", "video", "file", "prediction", "question", "goal", "list", "link"}

// Upload is a picture, a recording or a file kept in a capsule.
type Upload struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"user_id"`
	CapsuleID int64  `json:"capsule_id"`
	Name      string `json:"name"`
	Mime      string `json:"mime"`
	Bytes     int    `json:"bytes"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Data      []byte `json:"-"`
	Thumb     []byte `json:"-"`
	CreatedAt string `json:"created_at"`
}

// Event is one line of a capsule's history.
type Event struct {
	ID        int64  `json:"id"`
	CapsuleID int64  `json:"capsule_id"`
	UserID    *int64 `json:"user_id"`
	Who       string `json:"who"`
	What      string `json:"what"`
	Detail    string `json:"detail"`
	At        string `json:"at"`

	Title string `json:"title,omitempty"` // the capsule's, in lists across capsules
	Emoji string `json:"emoji,omitempty"`
}

// Ritual is a yearly nudge to write the next capsule.
type Ritual struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"-"`
	Title     string `json:"title"`
	Template  string `json:"template"`
	Month     int    `json:"month"`
	Day       int    `json:"day"`
	Years     int    `json:"years"`
	Active    bool   `json:"active"`
	LastYear  int    `json:"last_year"`
	CreatedAt string `json:"created_at"`
}

// Conn is one person's connection to a sibling Pocket app.
type Conn struct {
	Mode      string `json:"mode,omitempty"`
	URL       string `json:"url,omitempty"`
	Token     string `json:"token,omitempty"`
	PublicURL string `json:"public_url,omitempty"`
}

// Settings are the box-wide values plus one person's preferences and their
// own connections.
type Settings struct {
	Title         string `json:"title"`
	SetupComplete bool   `json:"setup_complete"`

	DateFormat string `json:"date_format"`
	TimeFormat string `json:"time_format"`
	WeekStart  int    `json:"week_start"` // 1 = Monday, 7 = Sunday

	// New capsules.
	OpenHour   int      `json:"open_hour"`   // a capsule opens at this hour of its day
	RemindDays int      `json:"remind_days"` // a reminder this many days before (0 = none)
	Sections   []string `json:"sections"`    // what of "the world today" a new capsule keeps
	Surprise   bool     `json:"surprise"`    // what others add stays hidden until the opening
	Slot       bool     `json:"slot"`        // things can still be dropped in after sealing

	// Other apps.
	CalendarOnSeal bool   `json:"calendar_on_seal"` // put the opening day in Pocket Calendar when sealing
	AlertsOnReady  bool   `json:"alerts_on_ready"`  // tell Pocket Alerts when a capsule is ready
	NotesFolder    string `json:"notes_folder"`     // where opened capsules go in Pocket Notes
	FilesFolder    string `json:"files_folder"`     // where copies go in Pocket Files
	ChatTo         string `json:"chat_to"`          // the Pocket Chat conversation openings are shared in
	AIHelp         bool   `json:"ai_help"`

	// Notifications.
	NotifyReady  bool   `json:"notify_ready"`  // a capsule can be opened
	NotifyRemind bool   `json:"notify_remind"` // a capsule opens soon
	NotifyAdded  bool   `json:"notify_added"`  // someone added to a capsule of yours
	NotifyOpened bool   `json:"notify_opened"` // someone opened a capsule you are in
	NotifyYearly bool   `json:"notify_yearly"` // time for a yearly capsule
	NotifyHour   int    `json:"notify_hour"`   // reminders go at this hour
	NotifyURL    string `json:"notify_url"`
	NotifyFormat string `json:"notify_format"` // apprise | ntfy | json

	// Each sibling Pocket app, by slug.
	Conns map[string]Conn `json:"conns"`

	// AI: Pocket AI through Core, or an OpenAI-compatible API with the
	// person's own key.
	AIProvider    string `json:"ai_provider"` // auto | pocket | openai
	AIPocketModel string `json:"ai_pocket_model"`
	OpenAIKey     string `json:"openai_api_key"`
	OpenAIModel   string `json:"openai_model"`
	OpenAIBaseURL string `json:"openai_base_url"`
}

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
