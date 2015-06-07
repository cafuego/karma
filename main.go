package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"regexp"

	"github.com/nickschuch/karma/storage"
	_ "github.com/nickschuch/karma/storage/memory"
	_ "github.com/nickschuch/karma/storage/dynamodb"
)

var (
	cliToken   = kingpin.Flag("token", "The Docker auth username (private repos).").Default("").OverrideDefaultFromEnvar("KARMA_TOKEN").String()
	cliTrigger = kingpin.Flag("trigger", "The Docker auth username (private repos).").Default("karma").OverrideDefaultFromEnvar("KARMA_TRIGGER").String()
	cliBackend = kingpin.Flag("storage", "The Docker auth username (private repos).").Default("memory").OverrideDefaultFromEnvar("KARMA_STORAGE").String()
)

type Response struct {
	Text string `json:"text"`
}

func main() {
	http.HandleFunc("/", handler)
	http.ListenAndServe(":8081", nil)
}

func handler(w http.ResponseWriter, r *http.Request) {
	// Get the values posted from Slack.
	r.ParseForm()

	// We need to ensure the the request has the correct token. Otherwise anyone
	// can steal our karma!
	token := r.Form.Get("token")
	if *cliToken != token {
			log.Println("Invalid token", token)
			return
	}

	// Slack has a concept of a "Trigger Word" when making a bot available to all
	// rooms. This is to ensure the string is meant for this bot.
	trigger := r.Form.Get("trigger_word")
	if cliTrigger != trigger {
			return
	}

	// Now we attempt to find out which user this request is for.
	text := r.Form.Get("text")
	slice := strings.Split(text, " ")
	if len(slice[1]) <= 0  {
		log.Println("Cannot find the user")
	}
	phrase := slice[1]
	user, err := getUser(phrase)
	if err != nil {
		log.Println("Cannot find the user: ", err)
		return
	}

	// Now that we have gone through all the check we can connect to the backend.
	s, err := storage.New(cliBackend)
	if err != nil {
		log.Println("Cannot start the backend: %v", cliBackend)
	}

	// Check for increase request.
	amount := increaseAmount(phrase)
	if amount > 0 {
		s.Increase(user, amount)
		return
	}

	// Check for decrease request.
	amount = decreaseAmount(phrase)
	if amount > 0 {
		s.Decrease(user, amount)
		return
	}

	// By this stage I think we can assume the user wants the amount associated
	// with a user.
	amount = s.Get(user)
	response := Response{user + " = " + strconv.Itoa(amount)}
	js, err := json.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

// Passes the text and looks for a username.
func getUser(t string) (string, error) {
		// Remove all except for characters.
		reg, err := regexp.Compile("[^A-Za-z]+")
		if err != nil {
			log.Fatal(err)
		}
		safe := reg.ReplaceAllString(t, "")
		safe = strings.ToLower(strings.Trim(safe, "-"))
    return safe, nil
}

// Check if the text asked for an increase.
func increaseAmount(t string) int {
	// If the user gets a ++ result.
	if strings.Contains(t, "++") {
		return 1
	}

	// If the user gets a += result.
	if strings.Contains(t, "+=") {
		return findMultiAmount(t)
	}

	return 0
}

// Check if the text asked for a decrease.
func decreaseAmount(t string) int {
	// If the user gets a -- result.
	if strings.Contains(t, "--") {
		return 1
	}

	// If the user gets a -= result.
	if strings.Contains(t, "-=") {
		return findMultiAmount(t)
	}

	return 0
}

// Common handler for "+=" and "-=" strings.
func findMultiAmount(t string) int {
	slice := strings.Split(t, "=")
	// Ensure there is a value.
	if len(slice[1]) > 0  {
		// Ensure we don't have any unwanted characters.
		reg, err := regexp.Compile("[^0-9]+")
		if err != nil {
			log.Fatal(err)
		}
		replaced := reg.ReplaceAllString(slice[1], "")

		// Convert it to an int for calcuating.
		value, err := strconv.Atoi(replaced)
    if err != nil {
			return 0
		}
		return value
	}

	return 0
}
