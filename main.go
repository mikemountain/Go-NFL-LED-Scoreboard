package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/tidwall/gjson"
)

const URL string = "http://site.api.espn.com/apis/site/v2/sports/football/nfl/scoreboard"

type Game struct {
	EventId   string
	EventName string
	Date      string
	InitTime  time.Time
	Updated   time.Time

	DownDistance string
	Spot         string
	Redzone      bool
	Possession   string // this is the team ID in string form -_-
	GameClock    string
	Quarter      int
	Completed    bool
	State        string
	ScoringEvent string

	HomeTeam  string
	HomeId    string
	HomeScore string

	AwayTeam  string
	AwayId    string
	AwayScore string
}

func initGames(games map[string]*Game, q *[]string) {
	dt := time.Now()

	res, err := http.Get(URL)
	if err != nil {
		fmt.Printf("error making http request: %s\n", err)
		os.Exit(1)
	}
	// fmt.Printf("Got status code: %d\n", res.StatusCode)
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)

	events := gjson.GetBytes(body, "events")
	events.ForEach(func(key, value gjson.Result) bool {
		results := gjson.GetMany(value.Raw,
			// event data
			"id",
			"shortName",
			"date",
			"competitions.0.situation.shortDownDistanceText",
			"competitions.0.situation.possessionText",
			"competitions.0.situation.isRedZone",
			"competitions.0.situation.possession",
			"competitions.0.status.displayClock",
			"competitions.0.status.period",
			"competitions.0.status.type.completed",
			"competitions.0.status.type.state",
			// home team
			"competitions.0.competitors.0.team.abbreviation",
			"competitions.0.competitors.0.team.id",
			"competitions.0.competitors.0.score",
			// away team
			"competitions.0.competitors.1.team.abbreviation",
			"competitions.0.competitors.1.team.id",
			"competitions.0.competitors.1.score",
		)

		possession := results[11].Str
		if results[6].Str == results[15].Str {
			possession = results[14].Str
		}

		game := &Game{
			EventId:   results[0].Str,
			EventName: results[1].Str,
			Date:      results[2].Str,
			InitTime:  dt,
			Updated:   dt,

			DownDistance: results[3].Str,
			Spot:         results[4].Str,
			Redzone:      results[5].Bool(),
			Possession:   possession,
			GameClock:    results[7].Str,
			Quarter:      int(results[8].Num), // might need to be string, forget how OT is handled
			Completed:    results[9].Bool(),
			State:        results[10].Str,

			HomeTeam:  results[11].Str,
			HomeId:    results[12].Str,
			HomeScore: results[13].Str,

			AwayTeam:  results[14].Str,
			AwayId:    results[15].Str,
			AwayScore: results[16].Str,
		}

		games[results[0].Str] = game
		*q = append(*q, results[0].Str)
		return true // keep iterating
	})
}

func updateGames(games map[string]*Game, priority chan<- string, t int) {
	res, err := http.Get(URL)
	if err != nil {
		fmt.Printf("error making http request: %s\n", err)
		os.Exit(1)
	}
	// fmt.Printf("Got status code: %d\n", res.StatusCode)
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	events := gjson.GetBytes(body, "events")
	events.ForEach(func(key, value gjson.Result) bool {
		results := gjson.GetMany(value.Raw,
			// event data
			"id",
			"competitions.0.situation.shortDownDistanceText",
			"competitions.0.situation.possessionText",
			"competitions.0.situation.isRedZone",
			"competitions.0.situation.possession",
			"competitions.0.status.displayClock",
			"competitions.0.status.period",
			"competitions.0.status.type.completed",
			"competitions.0.status.type.state",
			// home team
			"competitions.0.competitors.0.team.abbreviation",
			"competitions.0.competitors.0.team.id",
			"competitions.0.competitors.0.score",
			// away team
			"competitions.0.competitors.1.team.abbreviation",
			"competitions.0.competitors.1.team.id",
			"competitions.0.competitors.1.score",
			// scoring event?
			"competitions.0.situation.lastPlay.type.abbreviation",
		)

		game := games[results[0].Str]

		possession := results[9].Str
		if results[4].Str == results[13].Str {
			possession = results[12].Str
		}

		game.Updated = time.Now()

		game.DownDistance = results[1].Str
		game.Spot = results[2].Str
		game.Redzone = results[3].Bool()
		game.Possession = possession
		game.GameClock = results[5].Str
		game.Quarter = int(results[6].Num) // might need to be string, forget how OT is handle
		game.Completed = results[7].Bool()
		game.State = results[8].Str

		game.HomeScore = results[11].Str
		game.AwayScore = results[14].Str

		game.ScoringEvent = results[15].Str
		if results[15].Str == "FG" || results[15].Str == "TD" {
			priority <- results[0].Str
		}
		// change this to scoring event put on channel
		// if t%3 == 0 && id.Str == "401437777" {
		// 	fmt.Println("putting on channel", t)
		// 	priority <- id.Str
		// }
		return true // keep iterating
	})
}

func rotate(q []string) []string {
	id, q := q[0], q[1:]
	q = append(q, id)
	return q
}

func main() {
	games := make(map[string]*Game)
	priority := make(chan string)
	q := []string{}

	initGames(games, &q)
	// fmt.Printf("%+v\n", games["401437777"])
	// time.Sleep(1000 * time.Millisecond)

	// AGAIN
	// updateGames(games)
	// fmt.Printf("%+v\n", games["401437777"])
	// fmt.Println(q)
	t := time.NewTicker(3 * time.Second)
	i := 0
	for _ = range t.C {
		go updateGames(games, priority, i)
		select {
		case g := <-priority:
			fmt.Println("scoring event in", g)
		default:
			fmt.Println("no message received")
			q = rotate(q)
			fmt.Println(q)
		}
		i++
	}
}
