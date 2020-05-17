package handler_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/steinfletcher/apitest"
	jsonpath "github.com/steinfletcher/apitest-jsonpath"

	"github.com/hiconvo/api/testutil"
)

func TestCreateUser(t *testing.T) {
	existingUser, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	incompleteUser := testutil.NewIncompleteUser(_ctx, t, _dbClient, _searchClient)

	tests := []struct {
		Name         string
		GivenBody    map[string]interface{}
		ExpectStatus int
		ExpectBody   string
	}{
		{
			Name: "success",
			GivenBody: map[string]interface{}{
				"email":     "ruth.marcus@yale.edu",
				"firstName": "Ruth",
				"lastName":  "Marcus",
				"password":  "the comma is a giveaway",
			},
			ExpectStatus: http.StatusCreated,
			ExpectBody:   "",
		},
		{
			Name: "need to verify",
			GivenBody: map[string]interface{}{
				"email":     incompleteUser.Email,
				"firstName": "Thomas",
				"lastName":  "Aquinas",
				"password":  "angels are real!",
			},
			ExpectStatus: http.StatusOK,
			ExpectBody:   `{"message": "Please verify your email to proceed"}`,
		},
		{
			Name: "missing name and password",
			GivenBody: map[string]interface{}{
				"email":    "rudolf.carnap@charles.cz",
				"lastName": "Carnap",
				"password": "",
			},
			ExpectStatus: http.StatusBadRequest,
			ExpectBody:   `{"firstName":"This field is required","password":"Must be at least 8 characters long"}`,
		},
		{
			Name: "type mismatch",
			GivenBody: map[string]interface{}{
				"email":     "kit.fine@nyu.edu",
				"firstName": true,
				"password":  "Reality is constituted by tensed facts",
			},
			ExpectStatus: http.StatusBadRequest,
			ExpectBody:   `{"message":"Could not decode JSON"}`,
		},
		{
			Name: "already registered",
			GivenBody: map[string]interface{}{
				"email":     existingUser.Email,
				"firstName": "Ruth",
				"lastName":  "Millikan",
				"password":  "Language and thought are biological categories",
			},
			ExpectStatus: http.StatusBadRequest,
			ExpectBody:   `{"message":"This email has already been registered"}`,
		},
		{
			Name: "invalid email",
			GivenBody: map[string]interface{}{
				"email":     "it's all in my mind",
				"firstName": "George",
				"lastName":  "Berkeley",
				"password":  "Ordinary objects are ideas",
			},
			ExpectStatus: http.StatusBadRequest,
			ExpectBody:   `{"email":"This is not a valid email"}`,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Post("/users").
				JSON(tcase.GivenBody).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.ExpectStatus == http.StatusOK {
				tt.Body(tcase.ExpectBody)
			} else if tcase.ExpectStatus < http.StatusBadRequest {
				tt.Assert(jsonpath.Equal("$.email", tcase.GivenBody["email"].(string))).
					Assert(jsonpath.Equal("$.firstName", tcase.GivenBody["firstName"].(string))).
					Assert(jsonpath.Equal("$.lastName", tcase.GivenBody["lastName"].(string)))
			} else {
				tt.Body(tcase.ExpectBody)
			}

			tt.End()
		})
	}
}

func TestAuthenticateUser(t *testing.T) {
	existingUser, password := testutil.NewUser(_ctx, t, _dbClient, _searchClient)

	tests := []struct {
		Name         string
		GivenBody    map[string]interface{}
		ExpectStatus int
		ExpectBody   string
	}{
		{
			Name: "success",
			GivenBody: map[string]interface{}{
				"email":    existingUser.Email,
				"password": password,
			},
			ExpectStatus: http.StatusOK,
		},
		{
			Name: "invalid password",
			GivenBody: map[string]interface{}{
				"email":    existingUser.Email,
				"password": "123456789",
			},
			ExpectStatus: http.StatusBadRequest,
			ExpectBody:   `{"message":"Invalid credentials"}`,
		},
		{
			Name: "missing password",
			GivenBody: map[string]interface{}{
				"email":    existingUser.Email,
				"password": "",
			},
			ExpectStatus: http.StatusBadRequest,
			ExpectBody:   `{"password":"This field is required"}`,
		},
		{
			Name: "invalid password again",
			GivenBody: map[string]interface{}{
				"email":    "santa@northpole.com",
				"password": "have you been naughty or nice?",
			},
			ExpectStatus: http.StatusBadRequest,
			ExpectBody:   `{"message":"Invalid credentials"}`,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Post("/users/auth").
				JSON(tcase.GivenBody).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.ExpectStatus >= http.StatusBadRequest {
				tt.Body(tcase.ExpectBody)
			} else {
				tt.Assert(jsonpath.Equal("$.id", existingUser.ID))
				tt.Assert(jsonpath.Equal("$.firstName", existingUser.FirstName))
				tt.Assert(jsonpath.Equal("$.lastName", existingUser.LastName))
				tt.Assert(jsonpath.Equal("$.fullName", existingUser.FullName))
				tt.Assert(jsonpath.Equal("$.token", existingUser.Token))
				tt.Assert(jsonpath.Equal("$.verified", existingUser.Verified))
				tt.Assert(jsonpath.Equal("$.email", existingUser.Email))
			}

			tt.End()
		})
	}
}

func TestGetCurrentUser(t *testing.T) {
	existingUser, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)

	tests := []struct {
		Name            string
		GivenAuthHeader map[string]string
		ExpectStatus    int
		ExpectBody      string
	}{
		{
			Name:            "success",
			GivenAuthHeader: map[string]string{"Authorization": fmt.Sprintf("Bearer %s", existingUser.Token)},
			ExpectStatus:    http.StatusOK,
		},
		{
			Name:            "bad token",
			GivenAuthHeader: map[string]string{"Authorization": "Bearer abcdefghijklmnopqrstuvwxyz"},
			ExpectStatus:    http.StatusUnauthorized,
			ExpectBody:      `{"message":"Unauthorized"}`,
		},
		{
			Name:            "invalid header",
			GivenAuthHeader: map[string]string{"everything": "is what it is"},
			ExpectStatus:    http.StatusUnauthorized,
			ExpectBody:      `{"message":"Unauthorized"}`,
		},
		{
			Name:            "missing header",
			GivenAuthHeader: nil,
			ExpectStatus:    http.StatusUnauthorized,
			ExpectBody:      `{"message":"Unauthorized"}`,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Get("/users").
				Headers(tcase.GivenAuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.ExpectStatus >= http.StatusBadRequest {
				tt.Body(tcase.ExpectBody)
			} else {
				tt.Assert(jsonpath.Equal("$.id", existingUser.ID))
				tt.Assert(jsonpath.Equal("$.firstName", existingUser.FirstName))
				tt.Assert(jsonpath.Equal("$.lastName", existingUser.LastName))
				tt.Assert(jsonpath.Equal("$.token", existingUser.Token))
				tt.Assert(jsonpath.Equal("$.verified", existingUser.Verified))
				tt.Assert(jsonpath.Equal("$.email", existingUser.Email))
			}

			tt.End()
		})
	}
}

func TestGetUser(t *testing.T) {
	existingUser, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	user1, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)

	tests := []struct {
		Name            string
		GivenAuthHeader map[string]string
		URL             string
		ExpectStatus    int
		ExpectBody      string
	}{
		{
			Name:            "success",
			GivenAuthHeader: map[string]string{"Authorization": fmt.Sprintf("Bearer %s", existingUser.Token)},
			URL:             fmt.Sprintf("/users/%s", user1.ID),
			ExpectStatus:    http.StatusOK,
		},
		{
			Name:            "bad token",
			GivenAuthHeader: map[string]string{"Authorization": "Bearer abcdefghijklmnopqrstuvwxyz"},
			URL:             fmt.Sprintf("/users/%s", user1.ID),
			ExpectStatus:    http.StatusUnauthorized,
			ExpectBody:      `{"message":"Unauthorized"}`,
		},
		{
			Name:            "bad url",
			GivenAuthHeader: map[string]string{"Authorization": fmt.Sprintf("Bearer %s", existingUser.Token)},
			URL:             fmt.Sprintf("/users/%s", "somenonsense"),
			ExpectStatus:    http.StatusNotFound,
			ExpectBody:      `{"message":"The requested resource was not found"}`,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Get(tcase.URL).
				Headers(tcase.GivenAuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.ExpectStatus >= http.StatusBadRequest {
				tt.Body(tcase.ExpectBody)
			} else {
				tt.Assert(jsonpath.Equal("$.id", user1.ID))
				tt.Assert(jsonpath.Equal("$.firstName", user1.FirstName))
				tt.Assert(jsonpath.Equal("$.lastName", user1.LastName))
				tt.Assert(jsonpath.Equal("$.fullName", user1.FullName))
				tt.Assert(jsonpath.NotPresent("$.token"))
				tt.Assert(jsonpath.NotPresent("$.verified"))
				tt.Assert(jsonpath.NotPresent("$.email"))
			}

			tt.End()
		})
	}
}

func TestOAuth(t *testing.T) {
	existingUser1, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	existingUser2, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	existingUser2.PasswordDigest = ""
	existingUser2.Verified = false
	if _, err := _dbClient.Put(_ctx, existingUser2.Key, existingUser2); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		Name            string
		GivenBody       string
		GivenOAuthToken string
		GivenEmail      string
		ExpectStatus    int
		ExpectFirstName string
		ExpectLastName  string
		Token           string
	}{
		{
			Name:            "success",
			GivenOAuthToken: "123",
			GivenEmail:      "bob.kennedy@whitehouse.gov",
			GivenBody:       `{"provider": "google", "token": "123"}`,
			ExpectStatus:    200,
			ExpectFirstName: "John",
			ExpectLastName:  "Kennedy",
		},
		{
			Name:            "success",
			GivenOAuthToken: "123",
			GivenEmail:      "bob.kennedy@whitehouse.gov",
			GivenBody:       `{"provider": "google", "token": "123"}`,
			ExpectStatus:    200,
			ExpectFirstName: "John",
			ExpectLastName:  "Kennedy",
		},
		{
			Name:            "success with existing user",
			GivenOAuthToken: "456",
			GivenEmail:      existingUser1.Email,
			GivenBody:       `{"provider": "google", "token": "456"}`,
			ExpectStatus:    200,
			ExpectFirstName: existingUser1.FirstName,
			ExpectLastName:  existingUser1.LastName,
		},
		{
			Name:            "success and merge with existing user",
			GivenOAuthToken: "789",
			GivenEmail:      "merge@me.com",
			GivenBody:       `{"provider": "google", "token": "789"}`,
			ExpectStatus:    200,
			ExpectFirstName: existingUser2.FirstName,
			ExpectLastName:  existingUser2.LastName,
			Token:           existingUser2.Token,
		},
		{
			Name:            "invalid token",
			GivenOAuthToken: "789",
			GivenEmail:      "merge@me.com",
			GivenBody:       `{"provider": "notvalid", "token": "notvalid"}`,
			ExpectStatus:    400,
			Token:           existingUser2.Token,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			oauthMock := apitest.NewMock().
				Get(fmt.Sprintf("https://oauth2.googleapis.com/tokeninfo?id_token=%s", tcase.GivenOAuthToken)).
				RespondWith().
				Body(fmt.Sprintf(`{
					"aud": "",
					"sub": "%s",
					"email": "%s",
					"given_name": "%s",
					"family_name": "%s",
					"picture": ""
				}`, tcase.GivenEmail, tcase.GivenEmail, tcase.ExpectFirstName, tcase.ExpectLastName)).
				Status(200).
				End()

			headers := map[string]string{"Content-Type": "application/json"}

			if tcase.Token != "" {
				headers["Authorization"] = fmt.Sprintf("Bearer %s", tcase.Token)
			}

			tt := apitest.New("OAuth").
				Mocks(oauthMock).
				Handler(_handler).
				Post("/users/oauth").
				Headers(headers).
				Body(tcase.GivenBody).
				Expect(t).
				Status(tcase.ExpectStatus)
			if tcase.ExpectStatus < 300 {
				tt.Assert(jsonpath.Equal("$.email", tcase.GivenEmail))
				tt.Assert(jsonpath.Equal("$.firstName", tcase.ExpectFirstName))
				tt.Assert(jsonpath.Equal("$.lastName", tcase.ExpectLastName))
			}

			tt.End()

		})
	}
}
