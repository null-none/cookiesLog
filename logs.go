package main

import (
    "github.com/itsjamie/gin-cors"
    "github.com/gin-gonic/gin"
    "gopkg.in/go-playground/validator.v8"
    "github.com/goamz/goamz/aws"
    "github.com/goamz/goamz/dynamodb"
    "time"
    "log"
    "strconv"
    "net"
    "net/http"
    "net/url"
    "fmt"
    "os"
    "crypto/md5"
    "encoding/hex"
)

type Visit struct {
    UID                       string     `validate:"required"`
    DateTime                  string     `validate:"required"`
    Resource_Id               string     `validate:"required"`
    Resource_User_Id          string     `validate:"required"`
    
    Event_Type                string     `validate:"required"`
    Event_Name                string     `validate:"required"`
    Event_Data                string     `validate:"required"`
    Event_Object_Name         string     `validate:"required"`
    Event_Object_Action       string     `validate:"required"`
    Event_Object_Action_Value string     `validate:"required"`

    User_ip                   string     `validate:"required"`
    User_Browser              string     `validate:"required"`
    User_Browser_Version      string     `validate:"required"`
    User_Device               string     `validate:"required"`
    User_Display              string     `validate:"required"`
    User_Flash                string     `validate:"required"`
    User_Lang                 string     `validate:"required"`
    User_Os                   string     `validate:"required"`
}


var validate *validator.Validate
const TIME_COOKIE int = 3600 * 366 * 5 * 24
const VERSION string = "1.1"

func getClientIPByRequest(req *http.Request) (ip string, err error) {
    ip, port, err := net.SplitHostPort(req.RemoteAddr)
    if err != nil {
        log.Printf("debug: Getting req.RemoteAddr %v", err)
        return "", err
    } else {
        log.Printf("debug: With req.RemoteAddr found IP:%v; Port: %v", ip, port)
    }

    userIP := net.ParseIP(ip)
    if userIP == nil {
        message := fmt.Sprintf("debug: Parsing IP from Request.RemoteAddr got nothing.")
        return "", fmt.Errorf(message)
    }
    log.Printf("debug: Found IP: %v", userIP)
    return userIP.String(), nil
}

func getMD5Hash(text string) string {
    hasher := md5.New()
    hasher.Write([]byte(text))
    return hex.EncodeToString(hasher.Sum(nil))
}

func main() {

    r := gin.Default()

    dir, err := os.Getwd()
    if err != nil {
      log.Fatal(err)
    }
    r.Static("/public", dir + "/public")

    r.Use(cors.Middleware(cors.Config{
        Origins:        "*",
        Methods:        "GET, PUT, POST, DELETE",
        RequestHeaders: "Origin, Authorization, Content-Type",
        ExposedHeaders: "",
        MaxAge: 50 * time.Second,
        Credentials: true,
        ValidateHeaders: false,
    }))

    r.GET("/uid", func(c *gin.Context) {
      var UID string
      if cookie, err := c.Request.Cookie("uid"); err == nil {
        UID = cookie.Value
      } else {
        var userId int
        current := int(time.Now().UnixNano())
        userId = current
        UID = url.QueryEscape(getMD5Hash(strconv.Itoa(userId)))
        http.SetCookie(c.Writer, &http.Cookie{
          Name:     "uid",
          Value:    UID,
          MaxAge:   TIME_COOKIE,
          Path:     "/",
        })

      }
      c.JSON(200, gin.H{
        "status": "ok",
        "version": VERSION,
        "uid": UID,
      })  

    })

    r.POST("/visit", func(c *gin.Context) {
        var UID string
        var current int
        ip, err := getClientIPByRequest(c.Request)

        if err != nil {
          ip = "localhost"
        } 

        if cookie, err := c.Request.Cookie("uid"); err == nil {
          current = int(time.Now().UnixNano())
          UID = cookie.Value
        } else {
          var userId int
          current := int(time.Now().UnixNano())
          userId = current
          UID = url.QueryEscape(getMD5Hash(strconv.Itoa(userId)))
          http.SetCookie(c.Writer, &http.Cookie{
            Name:     "uid",
            Value:    UID,
            MaxAge:   TIME_COOKIE,
            Path:     "/",
          })

        }

        auth, err := aws.EnvAuth()
        if err != nil {
            log.Panic(err)
        }

        config := &validator.Config{TagName: "validate"}
        validate = validator.New(config)

        visit := &Visit{
          UID: c.DefaultPostForm("UID", UID),
          DateTime:                     c.PostForm("DateTime"),
          Resource_Id:                  c.PostForm("Resource_Id"),
          Resource_User_Id:             c.DefaultPostForm("Resource_User_Id", "-"),
          Event_Type:                   c.PostForm("Event_Type"),
          Event_Name:                   c.DefaultPostForm("Event_Name", "-"),
          Event_Data:                   c.DefaultPostForm("Event_Data", "-"),
          Event_Object_Name:            c.DefaultPostForm("Event_Object_Name", "-"),
          Event_Object_Action:          c.DefaultPostForm("Event_Object_Action", "-"),
          Event_Object_Action_Value:    c.DefaultPostForm("Event_Object_Action_Value", "-"),
          User_ip:                      c.DefaultPostForm("User_ip", ip),
          User_Browser:                 c.PostForm("User_Browser"),
          User_Browser_Version:         c.PostForm("User_Browser_Version"),
          User_Device:                  c.PostForm("User_Device"),
          User_Display:                 c.PostForm("User_Display"),
          User_Flash:                   c.PostForm("User_Flash"),
          User_Lang:                    c.PostForm("User_Lang"),
          User_Os:                      c.PostForm("User_Os"),
        }

        errs := validate.Struct(visit)

        if errs != nil {
          c.JSON(400, gin.H{
              "message": errs,
              "status": "error",
              "version": VERSION,
          })
        } else {
          ddbs := dynamodb.Server{auth, aws.EUCentral}
          pkattr := dynamodb.NewStringAttribute("id", "")
        	pk := dynamodb.PrimaryKey{pkattr, nil}
        	table := dynamodb.Table{&ddbs, "visits", pk}
          ats, err := dynamodb.MarshalAttributes(visit)
          if err != nil {
              log.Panic(err)
          }

          row , err := table.PutItem(strconv.Itoa(current), strconv.Itoa(current), ats)
          if err != nil {
              log.Panic(err)
          }

          if row {
            c.JSON(200, gin.H{
                "status": "ok",
                "version": VERSION,
                "uid": UID,
                "id": current,
            })  
          } else {
              c.JSON(400, gin.H{
                  "message": "save problem",
                  "version": VERSION,
                  "status": "error",
              })              
            }
        }
    })
    r.Run(":80")
}
