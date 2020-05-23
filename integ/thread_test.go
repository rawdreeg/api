package handler_test

import (
	"fmt"
	"net/http"
	"testing"

	"cloud.google.com/go/datastore"
	"github.com/icrowley/fake"
	"github.com/steinfletcher/apitest"
	jsonpath "github.com/steinfletcher/apitest-jsonpath"
	"github.com/stretchr/testify/assert"

	"github.com/hiconvo/api/model"
	"github.com/hiconvo/api/testutil"
)

func TestCreateThread(t *testing.T) {
	u1, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	u2, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)

	tests := []struct {
		Name             string
		GivenAuthHeader  map[string]string
		GivenBody        map[string]interface{}
		ExpectStatus     int
		ExpectOwnerID    string
		ExpectMemberID   string
		ExpectMembersLen int
	}{
		{
			Name:            "success",
			GivenAuthHeader: testutil.GetAuthHeader(u1.Token),
			GivenBody: map[string]interface{}{
				"subject": fake.Title(),
				"users": []map[string]string{
					{
						"id": u2.ID,
					},
				},
			},
			ExpectStatus:     http.StatusCreated,
			ExpectOwnerID:    u1.ID,
			ExpectMemberID:   u2.ID,
			ExpectMembersLen: 2,
		},
		{
			Name:            "success with new users",
			GivenAuthHeader: testutil.GetAuthHeader(u1.Token),
			GivenBody: map[string]interface{}{
				"subject": fake.Title(),
				"users": []map[string]string{
					{
						"id": u2.ID,
					},
					{
						"email": "test@testing.com",
					},
					{
						"email": "test@testing.com",
					},
					{
						"email": "someone@somewhere.com",
					},
				},
			},
			ExpectStatus:     http.StatusCreated,
			ExpectOwnerID:    u1.ID,
			ExpectMemberID:   u2.ID,
			ExpectMembersLen: 4,
		},
		{
			Name:            "bad payload",
			GivenAuthHeader: testutil.GetAuthHeader(u1.Token),
			GivenBody: map[string]interface{}{
				"subject": fake.Title(),
				"users": []map[string]string{
					{
						"id": "Rudolf Carnap",
					},
				},
			},
			ExpectStatus: http.StatusBadRequest,
		},
		{
			Name:            "bad headers",
			GivenAuthHeader: map[string]string{"boop": "beep"},
			GivenBody: map[string]interface{}{
				"subject": fake.Title(),
				"users": []map[string]string{
					{
						"id": u2.ID,
					},
				},
			},
			ExpectStatus: http.StatusUnauthorized,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Post("/threads").
				JSON(tcase.GivenBody).
				Headers(tcase.GivenAuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.ExpectStatus < http.StatusBadRequest {
				tt.Assert(jsonpath.Equal("$.owner.id", tcase.ExpectOwnerID))
				tt.Assert(jsonpath.Contains("$.users[0].id", tcase.ExpectMemberID))
				tt.Assert(jsonpath.Len("$.users", tcase.ExpectMembersLen))
			}

			tt.End()
		})
	}
}

func TestGetThreads(t *testing.T) {
	owner, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	member1, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	member2, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	nonmember, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	thread := testutil.NewThread(_ctx, t, _dbClient, owner, []*model.User{member1, member2})

	tests := []struct {
		Name          string
		AuthHeader    map[string]string
		ExpectStatus  int
		IsThreadInRes bool
	}{
		{
			AuthHeader:    testutil.GetAuthHeader(owner.Token),
			ExpectStatus:  http.StatusOK,
			IsThreadInRes: true,
		},
		{
			AuthHeader:    testutil.GetAuthHeader(member1.Token),
			ExpectStatus:  http.StatusOK,
			IsThreadInRes: true,
		},
		{
			AuthHeader:    testutil.GetAuthHeader(member2.Token),
			ExpectStatus:  http.StatusOK,
			IsThreadInRes: true,
		},
		{
			AuthHeader:    testutil.GetAuthHeader(nonmember.Token),
			ExpectStatus:  http.StatusOK,
			IsThreadInRes: false,
		},
		{
			AuthHeader:    map[string]string{"boop": "beep"},
			ExpectStatus:  http.StatusUnauthorized,
			IsThreadInRes: false,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Get("/threads").
				Headers(tcase.AuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.IsThreadInRes {
				tt.Assert(jsonpath.Equal("$.threads[0].id", thread.ID))
				tt.Assert(jsonpath.Equal("$.threads[0].subject", thread.Subject))
				tt.Assert(jsonpath.Equal("$.threads[0].owner.id", owner.ID))
				tt.Assert(jsonpath.Equal("$.threads[0].users[0].id", member1.ID))
				tt.Assert(jsonpath.Equal("$.threads[0].users[1].id", member2.ID))
				tt.Assert(jsonpath.NotPresent("$.threads[0].users[0].email"))
				tt.Assert(jsonpath.NotPresent("$.threads[0].users[1].email"))
			}

			tt.End()
		})
	}
}

func TestGetThread(t *testing.T) {
	owner, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	member, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	nonmember, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	thread := testutil.NewThread(_ctx, t, _dbClient, owner, []*model.User{member})
	url := fmt.Sprintf("/threads/%s", thread.ID)

	tests := []struct {
		Name         string
		AuthHeader   map[string]string
		ExpectStatus int
	}{
		{
			AuthHeader:   testutil.GetAuthHeader(owner.Token),
			ExpectStatus: http.StatusOK,
		},
		{
			AuthHeader:   testutil.GetAuthHeader(member.Token),
			ExpectStatus: http.StatusOK,
		},
		{
			AuthHeader:   testutil.GetAuthHeader(nonmember.Token),
			ExpectStatus: http.StatusNotFound,
		},
		{
			AuthHeader:   map[string]string{"boop": "beep"},
			ExpectStatus: http.StatusUnauthorized,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Get(url).
				Headers(tcase.AuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.ExpectStatus < http.StatusBadRequest {
				tt.Assert(jsonpath.Equal("$.id", thread.ID))
				tt.Assert(jsonpath.Equal("$.subject", thread.Subject))
				tt.Assert(jsonpath.Equal("$.owner.id", owner.ID))
				tt.Assert(jsonpath.Equal("$.users[0].id", member.ID))
				tt.Assert(jsonpath.NotPresent("$.users[0].email"))
			}

			tt.End()
		})
	}
}

func TestDeleteThread(t *testing.T) {
	owner, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	member, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	nonmember, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	thread := testutil.NewThread(_ctx, t, _dbClient, owner, []*model.User{member})
	url := fmt.Sprintf("/threads/%s", thread.ID)

	tests := []struct {
		Name         string
		AuthHeader   map[string]string
		ExpectStatus int
		ShouldPass   bool
	}{
		{
			Name:         "member attempt",
			AuthHeader:   testutil.GetAuthHeader(member.Token),
			ExpectStatus: http.StatusNotFound,
			ShouldPass:   false,
		},
		{
			Name:         "nonmember attempt",
			AuthHeader:   testutil.GetAuthHeader(nonmember.Token),
			ExpectStatus: http.StatusNotFound,
			ShouldPass:   false,
		},
		{
			Name:         "invalid header",
			AuthHeader:   map[string]string{"boop": "beep"},
			ExpectStatus: http.StatusUnauthorized,
			ShouldPass:   false,
		},
		{
			Name:         "success",
			AuthHeader:   testutil.GetAuthHeader(owner.Token),
			ExpectStatus: http.StatusOK,
			ShouldPass:   true,
		},
		{
			Name:         "after success",
			AuthHeader:   testutil.GetAuthHeader(owner.Token),
			ExpectStatus: http.StatusNotFound,
			ShouldPass:   true,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			apitest.New(tcase.Name).
				Handler(_handler).
				Delete(url).
				Header("Content-Type", "application/json").
				Headers(tcase.AuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus).
				End()

			if tcase.ShouldPass {
				var gotThread model.Thread
				err := _dbClient.Get(_ctx, thread.Key, &gotThread)
				assert.Equal(t, datastore.ErrNoSuchEntity, err)
			}
		})
	}

}
