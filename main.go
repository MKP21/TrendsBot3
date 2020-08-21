package main

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/MKP21/anaconda"
	"github.com/sirupsen/logrus"
)

const (
	consumerKey       = "consumer key"
	consumerSecret    = "consumer secret"
	accessToken       = "accessToken"
	accessTokenSecret = "access Token Secret"
)

var (
	userMap = make(map[int64][]string)

	locationMap = map[string]int64{
		"New York USA":     2459115,
		"Los Angeles USA":  2442047,
		"Mumbai India":     2295411,
		"New Delhi India":  2295019,
		"London UK":        44418,
		"Sydney Australia": 1105779,
		"Toronto Canada":   4118,
	}
)

func main() {
	anaconda.SetConsumerKey(consumerKey)
	anaconda.SetConsumerSecret(consumerSecret)
	api := anaconda.NewTwitterApi(accessToken, accessTokenSecret)

	go dailyUpdateTask(api)

	//detect a mention
	stream := api.PublicStreamFilter(url.Values{
		"track": []string{"@TrendsBot3"},
	})

	defer stream.Stop()

	for v := range stream.C {
		tweet, ok := v.(anaconda.Tweet)
		if !ok {
			logrus.Warning("received unexpected type for value in mention")
			continue
		}
		location, woeid := processMention(tweet)

		updateMaps(tweet, location, woeid)

	}

}

func processMention(tweet anaconda.Tweet) (string, int64) {
	var location string
	var woeid string
	var sname []string

	if strings.Contains(tweet.Text, ":") {
		logrus.Warning("received tweet with invalid format")
		return "", 0
	}

	svalues := strings.Split(tweet.Text, ",")

	if len(svalues) == 2 {
		woeid = svalues[len(svalues)-1]
		sname = strings.Split((svalues[0]), " ")
	} else if len(svalues) == 1 {
		sname = strings.Split(tweet.Text, " ")
	} else {
		logrus.Warning("tweet format is invalid - Too many commas")
		return "", 0
	}

	sname = sname[1:]
	location = strings.Join(sname, " ")

	//trimming leading and trailing whitespaces
	location = strings.TrimSpace(location)
	woeid = strings.TrimSpace(woeid)

	nwoeid, err := strconv.ParseInt(woeid, 10, 64)

	if err != nil {
		logrus.Warning("tweet format is invalid - WOEID conversion or No WOEID")
		nwoeid = 0
	}

	return location, nwoeid
}

func updateMaps(tweet anaconda.Tweet, location string, woeid int64) {
	// contains locations currently subscribed - empty for new users
	values := []string{}

	if userVal, userok := userMap[tweet.User.Id]; userok {
		// retrieve existing locations given that the user already exists
		values = userVal
	}

	if location != "" {
		// check if location key exists already
		_, ok := locationMap[location]

		if ok {
			// location already present
			values = append(values, location)
			userMap[tweet.User.Id] = values
		} else if woeid != 0 {
			// new location, with a proper woeid
			locationMap[location] = woeid
			values = append(values, location)
			userMap[tweet.User.Id] = values
		} else {
			// a new location without a proper woeid
			logrus.Warning("invalid tweet format - provided a new location without a proper woeid")
		}
	} else {
		logrus.Warning("invalid tweet format - no location given")
	}

}

func dailyUpdateTask(api *anaconda.TwitterApi) {
	ticker := time.NewTicker(15 * time.Second)
	for _ = range ticker.C {
		trendMap := make(map[string]string)

		for userID, regionList := range userMap {

			messageText := "Good Morning :) \n \n"

			// check if we already have the trends for that region
			for _, region := range regionList {
				v, ok := trendMap[region]
				if ok {
					messageText = messageText + v + "\n"
					continue
				}

				// retrieve trends for a given region -> also needs tweet count
				result, _ := api.GetTrendsByPlace(locationMap[region], nil)
				year, month, day := time.Now().Date()
				trendText := "\n" + region + " (" + strconv.Itoa(day) + "-" + month.String() + "-" + strconv.Itoa(year) + ") : \n"
				for i, trend := range result.Trends {
					if strings.Contains(trend.Name, "#") {
						trendText += ".  " + trend.Name + "\n"
					} else {
						trendText += ". #" + trend.Name + "\n"
					}

					if i == 4 {
						break
					}
				}
				trendMap[region] = trendText
				messageText = messageText + trendText
			}

			// send message to the user
			messageText += "\n To unsubscribe reply with \"stop\""
			uid := strconv.FormatInt(userID, 10)
			helperMessage(uid, messageText)
		}

	}
}
