package main

import (
	"fmt"
	"os"
)

var (
	titles = []string{}
	user   = User{
		SessionID: "",
		UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_8_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/34.0.1847.116 Safari/537.36",
	}
)

func mkComments(text, punctuation, smiley []string) []string {
	res := []string{}
	for _, t := range text {
		for _, p := range punctuation {
			for _, s := range smiley {
				res = append(res, fmt.Sprintf("%s%s %s", t, p, s))
			}
			res = append(res, fmt.Sprintf("%s%s", t, p))
		}
		res = append(res, t)
	}
	return res
}

func main() {
	bot := Bot{
		User: &user,
		Comments: mkComments(
			[]string{"Thanks", "Thank you"},
			[]string{".", "!"},
			[]string{":)", ":D"},
		),
	}

	c := make(chan *Entry)
	go bot.EnterFromAll(titles, c)

	for {
		entry := <-c
		if entry == nil {
			os.Exit(0)
		} else if entry.Err != nil {
			fmt.Printf("%s: %s - %s\n", entry.Title, entry.URL, entry.Err)
		} else {
			fmt.Printf("%s: %s - %s\n", entry.Title, entry.URL, entry.Comment)
		}
	}
}
