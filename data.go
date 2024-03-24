package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"
)

// sample value
// {
// 	"secrets": [
// 	  {
// 		"Type": "regular",
// 		"Text": "goodPasswd : \"A)8hKd]xrcA33^6_...",
// 		"File": "/Credential.yaml",
// 		"RuleDesc": "Credential",
// 		"Suggestion": "Please cloak your password and secret key"
// 	  },
// 	  {
// 		"Type": "regular",
// 		"Text": "password : \"A)8hKd]xrcA33^6__B...",
// 		"File": "/Credential1.yaml",
// 		"RuleDesc": "Credential",
// 		"Suggestion": "Please cloak your password and secret key"
// 	  }
// 	],
// 	"set_ids": [
// 	  {
// 		"Type": "setgid",
// 		"File": "/var/log/apache2",
// 		"Evidence": "dgrwxr-xr-x"
// 	  },
// 	  {
// 		"Type": "setgid",
// 		"File": "/var/www/localhost/htdocs",
// 		"Evidence": "dgrwxr-xr-x"
// 	  },
// 	  {
// 		"Type": "setuid",
// 		"File": "/usr/sbin/suexec",
// 		"Evidence": "urwxr-xr-x"
// 	  }
// 	]
//}

// Secret represents a secret entry in the JSON
type Secret struct {
	Type       string `json:"Type"`
	Text       string `json:"Text"`
	File       string `json:"File"`
	RuleDesc   string `json:"RuleDesc"`
	Suggestion string `json:"Suggestion"`
}

// SetID represents a set_id entry in the JSON
type SetID struct {
	Type     string `json:"Type"`
	File     string `json:"File"`
	Evidence string `json:"Evidence"`
}

type DummyData struct {
	Secrets []Secret `json:"secrets"`
	SetIDs  []SetID  `json:"set_ids"`
}

// RandomJSON generates random JSON with the specified structure
func GenerateRandomJSON() string {
	rand.Seed(time.Now().UnixNano())

	secrets := []Secret{}
	setIDs := []SetID{}

	for i := 0; i < 2; i++ {
		secret := Secret{
			Type:       "regular",
			Text:       generateRandomString(20),
			File:       fmt.Sprintf("/Credential%d.yaml", i),
			RuleDesc:   "Credential",
			Suggestion: "Please cloak your password and secret key",
		}
		secrets = append(secrets, secret)
	}

	for i := 0; i < 3; i++ {
		setID := SetID{
			Type:     getRandomSetIDType(),
			File:     fmt.Sprintf("/var/log/apache%d", i+1),
			Evidence: generateRandomString(10),
		}
		setIDs = append(setIDs, setID)
	}

	data := &DummyData{
		Secrets: secrets,
		SetIDs:  setIDs,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return ""
	}

	return string(jsonData)
}

func generateRandomString(length int) string {
	charset := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*()"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}

func getRandomSetIDType() string {
	types := []string{"setgid", "setuid"}
	return types[rand.Intn(len(types))]
}
