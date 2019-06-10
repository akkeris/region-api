package app

import (
	"database/sql"
	"fmt"
	"os"
	structs "region-api/structs"
	utils "region-api/utils"
	"strconv"
	"strings"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

func GetSubscribers(params martini.Params, r render.Render) {
	app := params["app"]
	space := params["space"]
	servicegroup := "alamo-" + app + "-" + space + "-services"
	if space == "default" {
		servicegroup = "alamo-" + app + "-services"
	}
	subscribers, err := getSubscribers(servicegroup)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	r.JSON(200, subscribers)
}

func GetSubscribersDB(db *sql.DB, params martini.Params, r render.Render) {
	app := params["app"]
	space := params["space"]
	subscribers, err := getSubscribersDB(db, space, app)
	if err != nil {
		utils.ReportError(err, r)
		return
	}
	if len(subscribers) > 0 {
		r.JSON(200, subscribers)
	} else {
		empty := make([]int64, 0)
		r.JSON(200, empty)
	}
}

func RemoveSubscriber(spec structs.Subscriberspec, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	app := spec.Appname
	space := spec.Space
	email := spec.Email

	servicegroup := "alamo-" + app + "-" + space + "-services"
	if space == "default" {
		servicegroup = "alamo-" + app + "-services"
	}
	msg, err := removeSubscriber(servicegroup, email)
	if err != nil {
		fmt.Println(err)
		utils.ReportError(err, r)
		return
	}
	r.JSON(msg.Status, msg)
}

func RemoveSubscriberDB(db *sql.DB, spec structs.Subscriberspec, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	app := spec.Appname
	space := spec.Space
	email := spec.Email

	msg, err := removeSubscriberDB(db, space, app, email)
	if err != nil {
		fmt.Println(err)
		utils.ReportError(err, r)
		return
	}
	r.JSON(msg.Status, msg)
}

func AddSubscriberDB(db *sql.DB, spec structs.Subscriberspec, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	app := spec.Appname
	space := spec.Space
	email := spec.Email

	msg, err := addSubscriberDB(db, space, app, email)
	if err != nil {
		fmt.Println(err)
		utils.ReportError(err, r)
		return
	}
	r.JSON(msg.Status, msg)
}

func AddSubscriber(spec structs.Subscriberspec, berr binding.Errors, r render.Render) {
	if berr != nil {
		utils.ReportInvalidRequest(berr[0].Message, r)
		return
	}
	app := spec.Appname
	space := spec.Space
	email := spec.Email

	servicegroup := "alamo-" + app + "-" + space + "-services"
	if space == "default" {
		servicegroup = "alamo-" + app + "-services"
	}
	msg, err := addSubscriber(servicegroup, email)
	if err != nil {
		fmt.Println(err)
		utils.ReportError(err, r)
		return
	}
	r.JSON(msg.Status, msg)
}

func removeSubscriber(servicegroup string, email string) (msg structs.Messagespec, err error) {
	var toreturn structs.Messagespec
	var c *mgo.Collection
	session, err := mgo.Dial(os.Getenv("SUBSCRIPTION_URL"))
	if err != nil {
		panic(err)
	}
	c = session.DB("mydb").C("subscriptions")
	defer session.Close()
	var subscriber structs.Subscriber
	subscriber.Subscriber = email
	subscriber.Servicegroup = servicegroup
	changeinfo, err := c.RemoveAll(&subscriber)
	if err != nil {
		fmt.Println(err)
		return toreturn, err
	}

	toreturn.Status = 200
	toreturn.Message = "Removed :" + strconv.Itoa(changeinfo.Removed)
	return toreturn, nil

}

func removeSubscriberDB(db *sql.DB, space string, app string, email string) (msg structs.Messagespec, err error) {
	var toreturn structs.Messagespec
	stmt, err := db.Prepare("DELETE from subscribers where space=$1 and app=$2 and email=$3 ")
	if err != nil {
		fmt.Println(err)
		return toreturn, err
	}
	res, err := stmt.Exec(space, app, email)
	if err != nil {
		fmt.Println(err)
		return toreturn, err
	}
	_, err = res.RowsAffected()
	if err != nil {
		fmt.Println(err)
		return toreturn, err
	}

	toreturn.Status = 200
	toreturn.Message = "Removed :" + email
	return toreturn, nil

}

func addSubscriber(servicegroup string, email string) (msg structs.Messagespec, err error) {
	var toreturn structs.Messagespec

	exists := subscriberExists(servicegroup, email)
	if !exists {

		var c *mgo.Collection
		session, err := mgo.Dial(os.Getenv("SUBSCRIPTION_URL"))
		if err != nil {
			fmt.Println(err)
			return toreturn, err
		}
		c = session.DB("mydb").C("subscriptions")
		defer session.Close()
		var subscriber structs.Subscriber
		subscriber.Subscriber = email
		subscriber.Servicegroup = servicegroup
		err = c.Insert(&subscriber)
		if err != nil {
			fmt.Println(err)
			return toreturn, err
		}
	}
	toreturn.Status = 200
	toreturn.Message = "Added " + email + " to " + servicegroup
	return toreturn, nil

}

func addSubscriberDB(db *sql.DB, space string, app string, email string) (msg structs.Messagespec, err error) {
	var toreturn structs.Messagespec

	var emailret string
	inserterr := db.QueryRow("INSERT INTO subscribers(space,app,email) VALUES($1,$2,$3) returning email;", space, app, email).Scan(&emailret)
	if inserterr != nil {
		fmt.Println(inserterr)
		return toreturn, inserterr
	}

	toreturn.Status = 200
	toreturn.Message = "Added " + emailret + " to " + app + " in " + space
	return toreturn, nil

}

func subscriberExists(servicegroup string, email string) bool {
	var toreturn bool
	toreturn = false
	result, err := getSubscribers(servicegroup)
	if err != nil {
		fmt.Println(err)

	}
	for _, element := range result {
		fmt.Println(element.Subscriber)
		fmt.Println(element.Servicegroup)
		if strings.ToLower(element.Subscriber) == strings.ToLower(email) {
			toreturn = true
		}
	}
	return toreturn
}

func getSubscribers(servicegroup string) (subs []structs.Subscriber, err error) {

	var c *mgo.Collection
	session, err := mgo.Dial(os.Getenv("SUBSCRIPTION_URL"))
	if err != nil {
		return nil, err
	}
	c = session.DB("mydb").C("subscriptions")
	defer session.Close()
	result := []structs.Subscriber{}
	err = c.Find(bson.M{"servicegroup": servicegroup}).All(&result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func getSubscribersDB(db *sql.DB, space string, app string) (subs []structs.Subscriberspec, err error) {

	var result []structs.Subscriberspec
	stmt, err := db.Prepare("select email from subscribers where space=$1 and app=$2")
	if err != nil {
		fmt.Println(err)
		return result, err
	}
	defer stmt.Close()
	rows, err := stmt.Query(space, app)
	defer rows.Close()
	for rows.Next() {
		var email string
		err := rows.Scan(&email)
		if err != nil {
			fmt.Println(err)
			return result, err
		}
		var subscriber structs.Subscriberspec
		subscriber.Space = space
		subscriber.Appname = app
		subscriber.Email = email
		result = append(result, subscriber)
	}
	err = rows.Err()
	if err != nil {
		fmt.Println(err)
		return result, err
	}
	return result, nil
}
