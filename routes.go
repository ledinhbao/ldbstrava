package ldbstrava

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-contrib/location"
	"github.com/gin-gonic/contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"github.com/mitchellh/mapstructure"

	core "github.com/ledinhbao/ldbcore"
)

const (
	stravaAuthURL   = string("https://www.strava.com/oauth/authorize")
	tokenURL        = string("https://www.strava.com/oauth/token")
	revokeURL       = string("https://www.strava.com/oauth/deauthorize")
	subscriptionURL = string("https://www.strava.com/api/v3/push_subscriptions")
)

// InitializeRoutes inits routes with <prefix>/strava/*
func InitializeRoutes(engine *gin.Engine) {
	engine.Use(location.Default())
	stravaRoute := engine.Group(config.PathPrefix + "/strava")
	{
		stravaRoute.GET("/", stravaExchangeToken)
		stravaRoute.GET("/revoke/:username", stravaRevokeToken)
		stravaRoute.GET("/list/:username", listActivitiesForYesterday)
		stravaRoute.GET(config.PathSubscription, stravaValidateSubscription)
	}
}

func init() {
	config = &Config{
		ClientID:          "",
		ClientSecret:      "",
		PathPrefix:        "/admin",
		PathRedirect:      "/dashboard",
		PathSubscription:  "/subscription",
		GlobalDatabase:    "database",
		SubscriptionDBKey: "strava-subscription",
		URLCallbackHost:   "",
	}
}

func stravaExchangeToken(c *gin.Context) {
	code := c.Request.URL.Query().Get("code")
	if code == "" {
		// access denied
	} else {
		// Exchange for access token
		data := url.Values{}
		data.Set("client_id", config.ClientID)
		data.Set("client_secret", config.ClientSecret)
		data.Set("code", code)
		data.Set("grant_type", "authorization_code")

		client := &http.Client{}
		request, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
		response, err := client.Do(request)
		if err != nil {
			panic(err)
		}
		defer response.Body.Close()
		body, err := ioutil.ReadAll(response.Body)
		respData := make(map[string]interface{})
		err = json.Unmarshal(body, &respData)

		athlete := Athlete{}
		if athleteRaw, err := respData["athlete"]; err {
			mapstructure.Decode(athleteRaw, &athlete)
			// _ = json.Unmarshal([]byte(athleteRaw), &athlete)
		}

		session := sessions.Default(c)

		link := Link{}
		_ = mapstructure.Decode(respData, &link)
		link.Username = athlete.Username
		link.UserID = session.Get("AuthUserID").(uint)

		db := c.MustGet("database").(*gorm.DB)
		// check if link exist
		db.Where("username = ?", link.Username).Delete(Link{})
		db.Where("username = ?", athlete.Username).Delete(Athlete{})
		db.Create(&athlete)
		db.Create(&link)

		// c.JSON(http.StatusOK, gin.H{
		// 	"link":    link,
		// 	"athlete": athlete,
		// })
		c.Redirect(http.StatusFound, config.getRedirectPath())
	}

}

func getDatabaseInstance(c *gin.Context) *gorm.DB {
	return c.MustGet("database").(*gorm.DB)
}

func stravaRevokeToken(c *gin.Context) {
	// TODO Valid data where username is linked with user
	db := c.MustGet("database").(*gorm.DB)
	username := c.Param("username")
	link := Link{}
	_ = db.Where(Link{Username: username}).First(&link)

	client := &http.Client{}
	urlValues := url.Values{}
	urlValues.Set("access_token", link.AccessToken)

	request, _ := http.NewRequest("POST", revokeURL, strings.NewReader(urlValues.Encode()))
	response, _ := client.Do(request)

	if response.StatusCode >= 200 && response.StatusCode <= 299 {
		// Remove record from database
		go removeStravaRecord(db, username)
	}

	c.Redirect(http.StatusFound, config.getRedirectPath())
}

func removeStravaRecord(db *gorm.DB, username string) {
	tx := db.Begin()
	tx.Unscoped().Delete(Link{Username: username})
	tx.Unscoped().Delete(Athlete{Username: username})
	tx.Commit()
}

func listActivitiesForYesterday(c *gin.Context) {
	db := getDatabaseInstance(c)
	client := &http.Client{}
	var username = c.Param("username")
	var stravaLink = Link{}
	db.Where("username = ?", username).First(&stravaLink)
	var bearer = "Bearer " + stravaLink.AccessToken

	// TODO check for access_token expiration
	var request, _ = http.NewRequest("GET", "https://www.strava.com/api/v3/athlete/activities", nil)
	request.Header.Add("Authorization", bearer)
	resp, _ := client.Do(request)
	var body, _ = ioutil.ReadAll(resp.Body)
	c.JSON(http.StatusOK, gin.H{
		"data": string(body),
	})
}

func getSubscriptionToken() string {
	return config.ClientID + "hahaha" + config.ClientSecret
}

// createBaseURLData create url.Values content client_id and client_secret
func createBaseURLData() url.Values {
	res := url.Values{}
	res.Set("client_id", config.ClientID)
	res.Set("client_secret", config.ClientSecret)
	return res
}

func sendDeleteSubscription(subID string) {
	client := &http.Client{}
	urlData := createBaseURLData()
	req, _ := http.NewRequest("DELETE", subscriptionURL+"/"+subID, nil)
	req.URL.RawQuery = urlData.Encode()
	_, _ = client.Do(req)

	log.Println("Strava Delete Subscription URL sent:", req.URL.String())
}

// ViewSubscription send POST request to Strava server to get Subscription ID for your application
func ViewSubscription() string {
	client := &http.Client{}
	urlData := createBaseURLData()
	req, _ := http.NewRequest("GET", subscriptionURL, nil)
	req.URL.RawQuery = urlData.Encode()
	log.Println("View Subscription URL sent:", req.URL.String())
	resp, _ := client.Do(req)
	var jsonBody []map[string]interface{}
	body, _ := ioutil.ReadAll(resp.Body)
	json.Unmarshal(body, &jsonBody)

	if len(jsonBody) > 0 && resp.StatusCode == 200 {
		res := fmt.Sprintf("%v", jsonBody[0]["id"])
		log.Println("Subscription exist at Strava endpoint with ID: ", res, " ###")
		return res
	}
	log.Println("Subscription does not exist at Strava endpoint.")
	return ""
}

func sendSubscriptionCreationRequest() (int, map[string]interface{}) {
	client := &http.Client{}
	urlData := createBaseURLData()
	urlData.Set("verify_token", getSubscriptionToken())
	urlData.Set("callback_url", getCallbackURLOrPanic(true))
	req, _ := http.NewRequest("POST", subscriptionURL, nil)
	req.URL.RawQuery = urlData.Encode()
	// resp, _ := client.Post(subscriptionURL, "text/plain; charset=utf-8", strings.NewReader(urlData.Encode()))
	resp, _ := client.Do(req)
	body := make(map[string]interface{})
	json.NewDecoder(resp.Body).Decode(&body)
	log.Println("Strava Subscription Creation Request, URL:", body)
	return resp.StatusCode, body
}

// CreateSubscription kiểm tra trong bảng table config, kiểm tra SubscriptionDBKey có tồn tại hay không,
// nếu không có thì gởi POST request tới server Strava để đăng ký subscription.
// Cập nhật lại trường SubscriptionDBKey khi nhận dữ liệu về.
//
// Yêu cầu:
//     - CALLED AFTER SERVER RUN
//     - Package ledinhbao/core
//     - Bảng settings (key string, value string) trong database.
func CreateSubscription(db *gorm.DB) {
	log.Println("start... strava.CreateSubscription")
	setting := core.Setting{}
	notFoundSeting := db.Where(core.Setting{Key: config.SubscriptionDBKey}).First(&setting).RecordNotFound()

	if notFoundSeting || setting.Value == "" {
		// Un-sync between app and strava
		subscriptionID := ViewSubscription()
		if subscriptionID != "" {
			sendDeleteSubscription(subscriptionID)
		}

		// Send POST request and create database setting
		respCode, jsonBody := sendSubscriptionCreationRequest()
		if respCode == 201 {
			if subscriptionID, err := jsonBody["id"]; err {
				// Save subscription_id to database
				db.Where(
					core.Setting{Key: config.SubscriptionDBKey},
				).Assign(
					core.Setting{Value: fmt.Sprintf("%v", subscriptionID)},
				).FirstOrCreate(&setting)
			}
		} else {
			log.Panic("Error to create subscription with Strava")
		}
	}
}

func stravaValidateSubscription(c *gin.Context) {
	challenge := c.Query("hub.challenge")
	queryToken := c.Query("hub.verify_token")
	subscriptionToken := getSubscriptionToken()
	if queryToken == subscriptionToken {
		c.JSON(http.StatusOK, gin.H{
			"hub.challenge": challenge,
		})
	} else {
		c.JSON(http.StatusForbidden, gin.H{
			"query.token":    queryToken,
			"token.verified": subscriptionToken,
			"challenge":      challenge,
		})
	}
}
