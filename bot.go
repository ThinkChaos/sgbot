package main

import (
	"bytes"
	"errors"
	"github.com/PuerkitoBio/goquery"
	"io"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sync"
)

const (
	site_url   string = "http://www.steamgifts.com/"
	search_url string = site_url + "ajax_gifts.php"
)

type Entry struct {
	Comment string
	Err     error
	Title   string
	URL     string
}

type User struct {
	SessionID string
	UserAgent string

	formKey string
}

type Bot struct {
	client *http.Client
	User   *User

	Comments []string
}

func (b *Bot) newClient() error {
	c_url, _ := url.Parse(site_url)
	jar, _ := cookiejar.New(nil)
	jar.SetCookies(c_url, []*http.Cookie{&http.Cookie{
		Name:  "PHPSESSID",
		Value: b.User.SessionID,
	}})
	b.client = &http.Client{Jar: jar}
	return b.testSession()
}

func (b *Bot) testSession() error {
	req, err := b.newRequest("GET", site_url+"forum/new", nil)
	if err != nil {
		return err
	}
	b.client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return errors.New("bad login - http://steamgifts.com/?login")
	}
	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	b.client.CheckRedirect = nil

	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		return err
	}
	formKey, exists := doc.Find("#form_key").Attr("value")
	if !exists {
		return errors.New("form_key not found")
	}
	b.User.formKey = formKey

	return nil
}

func (b *Bot) newRequest(method, url string, body io.Reader) (*http.Request, error) {
	var req *http.Request
	if b.client == nil {
		err := b.newClient()
		if err != nil {
			return req, err
		}
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return req, err
	}
	if method == "POST" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.Header.Set("User-Agent", b.User.UserAgent)
	return req, nil
}

func (b *Bot) doRequest(method, url string, body io.Reader) (*http.Response, error) {
	req, err := b.newRequest(method, url, body)
	if err != nil {
		var resp *http.Response
		return resp, err
	}
	return b.client.Do(req)
}

func (b *Bot) doGET(url string) (*http.Response, error) {
	return b.doRequest("GET", url, nil)
}

func (b *Bot) doPOST(url, body string) (*http.Response, error) {
	return b.doRequest("POST", url, bytes.NewBufferString(body))
}

func (b *Bot) getHomepage() (*goquery.Document, error) {
	var doc *goquery.Document
	resp, err := b.doGET(site_url)
	if err != nil {
		return doc, err
	}
	return goquery.NewDocumentFromResponse(resp)
}

func (b *Bot) doSearch(title string) (*goquery.Document, error) {
	var doc *goquery.Document
	req, err := b.newRequest("POST", search_url, bytes.NewBufferString(
		"view=open&query="+url.QueryEscape(title),
	))
	if err != nil {
		return doc, err
	}
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	resp, err := b.client.Do(req)
	if err != nil {
		return doc, err
	}
	return goquery.NewDocumentFromResponse(resp)
}

func (b *Bot) EnterFromHomepage(c chan *Entry) {
	_, err := b.getHomepage()
	if err != nil {
		c <- &Entry{Err: err}
		return
	}

	// doc.Find("").Each(
	// 	enterGiveaways(title, comments, c),
	// )
}

func (b *Bot) EnterFromSearch(title string, wg *sync.WaitGroup, c chan *Entry) {
	defer wg.Done()
	doc, err := b.doSearch(title)
	if err != nil {
		c <- &Entry{
			Err:   err,
			Title: title,
		}
		return
	}

	doc.Find("div.post:not(.fade) > div.left > div.title > a").Each(
		b.enterGiveaways(title, c),
	)
}

func (b *Bot) enterGiveaways(title string, c chan *Entry) func(int, *goquery.Selection) {
	len_comments := len(b.Comments)
	err_check := func(err error, url string) bool {
		if err != nil {
			entry := &Entry{
				Err:   err,
				Title: title,
			}
			if url != "" {
				entry.URL = url
			}
			c <- entry
			return true
		}
		return false
	}

	return func(_ int, sel *goquery.Selection) {
		url, exists := sel.Attr("href")
		if !exists || url == "" {
			err_check(errors.New("no url"), "")
			return
		}

		gw_url := site_url + url[1:]
		resp, err := b.doGET(gw_url)
		if err_check(err, gw_url) {
			return
		}

		doc, err := goquery.NewDocumentFromResponse(resp)
		if err_check(err, gw_url) {
			return
		}

		contrib, err := b.enterGiveaway(gw_url, doc)
		if contrib || err_check(err, gw_url) {
			return
		}

		comment := b.Comments[rand.Int()%len_comments]
		err = b.commentGiveaway(gw_url, comment)
		if err_check(err, gw_url) {
			return
		}

		c <- &Entry{
			Comment: comment,
			Title:   title,
			URL:     url,
		}
	}
}

func (b *Bot) enterGiveaway(gw_url string, doc *goquery.Document) (bool, error) {
	sel := doc.Find("#form_enter_giveaway > a.rounded.view.submit_entry")

	if sel.Length() == 0 {
		// Contibutor Only
		if doc.Find("#form_enter_giveaway > a.rounded.view").Length() != 0 {
			return true, nil
		}
		return false, errors.New("can't join")
	}

	_, err := b.doPOST(gw_url, "enter_giveaway=1&form_key="+b.User.formKey)
	return false, err
}

func (b *Bot) commentGiveaway(gw_url, comment string) error {
	_, err := b.doPOST(gw_url,
		"submit_comment=Submit Comment&parent_id=0&form_key="+b.User.formKey+"&body="+comment,
	)
	return err
}

func (b *Bot) EnterFromAll(titles []string, c chan *Entry) {
	// b.EnterFromHomepage(c)

	wg := &sync.WaitGroup{}
	for _, title := range titles {
		wg.Add(1)
		go b.EnterFromSearch(title, wg, c)
	}
	wg.Wait()
	c <- nil
}
