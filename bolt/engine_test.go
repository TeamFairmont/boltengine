// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/. 

package bolt

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TeamFairmont/boltengine/commandprocess"
	"github.com/TeamFairmont/boltshared/config"
	"github.com/TeamFairmont/boltshared/utils"
	"github.com/TeamFairmont/gabs"
	"github.com/stretchr/testify/assert"
)

func TestCreateTestEngine(t *testing.T) {
	testengine := CreateTestEngine("error")
	assert.NotNil(t, testengine, "Engine 1 was created successfully")
	go testengine.ListenAndServe()
	testengine2 := CreateTestEngine("error")
	assert.NotNil(t, testengine2, "Engine 2 was created successfully")
	go testengine2.ListenAndServe()
}

//test ExtractCallName
func TestExtractCallName(t *testing.T) {
	//flags that should be removed
	flags := []string{
		"/request/",
		"/task/",
		"/work/",
		"/form/",
	}
	notFlags := []string{
		"/zazzle/",
		"/1234/",
		"/!/",
	}
	domain := "http://www.test.com"
	path := "$arc/moo(p/baz&&inga^/abc"

	runExtractCMD(flags, domain, path, true, t)
	runExtractCMD(notFlags, domain, path, false, t)
	runExtractCMD(flags, domain, "", true, t) //test empty path to see if slice boundary issue

}

func runExtractCMD(flags []string, domain, path string, noErr bool, t *testing.T) {
	//test for each flag
	for _, flag := range flags {
		//inserts a flag into the url
		url := domain + flag + path
		//creats a new request with the above url
		r, _ := http.NewRequest("POST", url, nil)

		//assigns the output of ExtractCallName to s and err
		s, err := ExtractCallName(r)

		if noErr {
			//check if s matches the desired output
			assert.Equal(t, url[len(domain)+len(flag):], s, "ExtractCallName results should match ")
		} else {
			//if an eeror was returned by ExtractCallName PASS
			assert.NotNil(t, err, "Err should be not nill.")
		}
	}
}

func TestOutputRequest(t *testing.T) {
	testJSON, _ := gabs.ParseJSON([]byte(`{
		"someThing":{
			"v1/stuff": 111
	},
		"!@#$%&*()_+":{
			"blockThing": 111
},
"apploTree":{
	"v1/stuff": 111
},
"a1pploTree":{
	"v1/stuff": 111
},
"1apploTree":{
	"v1/stuff": 111
},
"rappleTree":{
	"v1/stuff": 111
},
		"apiCalls": {
			"v1/addProduct": {
				"resultTimeoutMs": 100,
				"cache": {
					"enabled": false,
					"expirationTimeSec": 600,
					"allowOverride": false
				},
				"requiredParams": {
					"someGlobalOption1": "string",
					"someGlobalOption2": "bool"
				}
			}
		}
	}`))
	//keys to use as filters inf call to FilterPayload
	keys := []string{"apiCalls",
		"zapiCalls",
		"otherThing",
		"apploTree",
		"!@#$%&*()_+",
	}
	//initializing structs for OutputRequest
	var e Engine
	var cfg config.Config
	e.Config = &cfg
	e.Config.Engine.PrettyOutput = true
	var c commandprocess.CommandProcess
	c.Payload = testJSON
	w := httptest.NewRecorder()
	//OutputRequest uses Fprint to write to w
	e.OutputRequest(w, &c, keys)
	getJSON(t, w, testJSON, keys)
	//test with nil filter keys
	keys = nil
	e.OutputRequest(w, &c, keys)
	getJSON(t, w, testJSON, keys)
}

func getJSON(t *testing.T, w *httptest.ResponseRecorder, testJSON *gabs.Container, keys []string) {
	returnedBytes, err := ioutil.ReadAll(w.Body)
	payload, err := gabs.ParseJSON(returnedBytes)
	children, err := payload.ChildrenMap()
	if err != nil {
		fmt.Println(err)
	}
	if keys != nil {
		//check to see that all children returned by OutputRequest are in the filter keys list
		for child := range children {
			assert.True(t, utils.StringInSlice(child, keys))
		}
	} else {
		//if keys are nil, there should be no changes from testJSON to payload- Nothing to filter out.
		assert.Equal(t, testJSON, payload, "should be true")
	}

}
