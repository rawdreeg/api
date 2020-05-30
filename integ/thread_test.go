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

func TestGetMessagesByThread(t *testing.T) {
	owner, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	member1, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	member2, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	nonmember, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	thread := testutil.NewThread(_ctx, t, _dbClient, owner, []*model.User{member1, member2})
	message1 := testutil.NewThreadMessage(_ctx, t, _dbClient, owner, thread)
	message2 := testutil.NewThreadMessage(_ctx, t, _dbClient, member1, thread)
	url := fmt.Sprintf("/threads/%s/messages", thread.ID)

	tests := []struct {
		Name         string
		AuthHeader   map[string]string
		ExpectStatus int
	}{
		{
			Name:         "Owner can get messages",
			AuthHeader:   testutil.GetAuthHeader(owner.Token),
			ExpectStatus: http.StatusOK,
		},
		{
			Name:         "Member can get messages",
			AuthHeader:   testutil.GetAuthHeader(member1.Token),
			ExpectStatus: http.StatusOK,
		},
		{
			Name:         "NonMember cannot get messages",
			AuthHeader:   testutil.GetAuthHeader(nonmember.Token),
			ExpectStatus: http.StatusNotFound,
		},
		{
			Name:         "Unauthenticated user cannot get messages",
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
				tt.Assert(jsonpath.Equal("$.messages[0].id", message1.ID))
				tt.Assert(jsonpath.Equal("$.messages[0].body", message1.Body))
				tt.Assert(jsonpath.Equal("$.messages[1].id", message2.ID))
				tt.Assert(jsonpath.Equal("$.messages[1].body", message2.Body))
			}

			tt.End()
		})
	}
}

func TestMarkThreadAsRead(t *testing.T) {
	owner, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	member, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	nonmember, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	thread := testutil.NewThread(_ctx, t, _dbClient, owner, []*model.User{member})
	url := fmt.Sprintf("/threads/%s/reads", thread.ID)

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
				Post(url).
				Header("Content-Type", "application/json").
				Headers(tcase.AuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.ExpectStatus < http.StatusBadRequest {
				tt.Assert(jsonpath.Equal("$.id", thread.ID))
				tt.Assert(jsonpath.Equal("$.reads[0].id", owner.ID))
			}

			tt.End()
		})
	}
}

func TestUpdateThread(t *testing.T) {
	owner, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	member, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	nonmember, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	thread := testutil.NewThread(_ctx, t, _dbClient, owner, []*model.User{member})
	url := fmt.Sprintf("/threads/%s", thread.ID)

	tests := []struct {
		Name         string
		AuthHeader   map[string]string
		ShouldPass   bool
		ExpectStatus int
		GivenBody    map[string]interface{}
	}{
		{
			AuthHeader:   testutil.GetAuthHeader(owner.Token),
			ExpectStatus: http.StatusOK,
			ShouldPass:   true,
			GivenBody:    map[string]interface{}{"subject": "Ruth Marcus"},
		},
		{
			AuthHeader:   testutil.GetAuthHeader(member.Token),
			ExpectStatus: http.StatusNotFound,
			ShouldPass:   false,
			GivenBody:    map[string]interface{}{"subject": "Ruth Marcus"},
		},
		{
			AuthHeader:   testutil.GetAuthHeader(nonmember.Token),
			ExpectStatus: http.StatusNotFound,
			ShouldPass:   false,
		},
		{
			AuthHeader:   map[string]string{"boop": "beep"},
			ExpectStatus: http.StatusUnauthorized,
			ShouldPass:   false,
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Patch(url).
				JSON(tcase.GivenBody).
				Headers(tcase.AuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.ExpectStatus <= http.StatusBadRequest {
				if tcase.ShouldPass {
					tt.Assert(jsonpath.Equal("$.subject", tcase.GivenBody["subject"]))
				} else {
					tt.Assert(jsonpath.Equal("$.subject", thread.Subject))
				}
			}

			tt.End()
		})
	}
}

func TestAddUserToThread(t *testing.T) {
	owner, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	member, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	memberToAdd, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	secondMemberToAdd, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	nonmember, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	thread := testutil.NewThread(_ctx, t, _dbClient, owner, []*model.User{member})

	tests := []struct {
		Name         string
		AuthHeader   map[string]string
		ExpectStatus int
		GivenUserID  string
		ShouldPass   bool
		ExpectNames  []string
	}{
		{
			Name:         "nonmember attempt to add",
			AuthHeader:   testutil.GetAuthHeader(nonmember.Token),
			ExpectStatus: http.StatusNotFound,
			GivenUserID:  memberToAdd.ID,
		},
		{
			Name:         "member without permission attempt to add",
			AuthHeader:   testutil.GetAuthHeader(member.Token),
			ExpectStatus: http.StatusNotFound,
			GivenUserID:  memberToAdd.ID,
		},
		{
			Name:         "bad auth header",
			AuthHeader:   map[string]string{"boop": "beep"},
			ExpectStatus: http.StatusUnauthorized,
			GivenUserID:  memberToAdd.ID,
		},
		{
			Name:         "success with existing user",
			AuthHeader:   testutil.GetAuthHeader(owner.Token),
			ExpectStatus: http.StatusOK,
			GivenUserID:  memberToAdd.ID,
			ExpectNames: []string{
				member.FullName,
				owner.FullName,
				memberToAdd.FullName,
			},
		},
		{
			Name:         "success with email",
			AuthHeader:   testutil.GetAuthHeader(owner.Token),
			ExpectStatus: http.StatusOK,
			GivenUserID:  "addedOnThe@fly.com",
			ExpectNames: []string{
				member.FullName,
				owner.FullName,
				memberToAdd.FullName,
				"addedonthe",
			},
		},
		{
			Name:         "second success with email",
			AuthHeader:   testutil.GetAuthHeader(owner.Token),
			ExpectStatus: http.StatusOK,
			GivenUserID:  secondMemberToAdd.Email,
			ExpectNames: []string{
				member.FullName,
				owner.FullName,
				memberToAdd.FullName,
				"addedonthe",
				secondMemberToAdd.FullName,
			},
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Post(fmt.Sprintf("/threads/%s/users/%s", thread.ID, tcase.GivenUserID)).
				JSON(`{}`).
				Headers(tcase.AuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.ExpectStatus <= http.StatusBadRequest {
				tt.Assert(jsonpath.Equal("$.id", thread.ID))
				tt.Assert(jsonpath.Equal("$.owner.id", owner.ID))
				tt.Assert(jsonpath.Equal("$.owner.fullName", owner.FullName))
				for i := range tcase.ExpectNames {
					tt.Assert(jsonpath.Equal(
						fmt.Sprintf("$.users[%d].fullName", i),
						tcase.ExpectNames[i]))
				}
			}

			tt.End()
		})
	}
}

func TestRemoveFromThread(t *testing.T) {
	owner, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	member, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	memberToRemove, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	memberToLeave, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	nonmember, _ := testutil.NewUser(_ctx, t, _dbClient, _searchClient)
	thread := testutil.NewThread(_ctx, t, _dbClient, owner, []*model.User{member, memberToRemove, memberToLeave})

	tests := []struct {
		Name              string
		AuthHeader        map[string]string
		GivenUserID       string
		ExpectStatus      int
		ExpectMemberIDs   []string
		ExpectMemberNames []string
	}{
		{
			Name:         "nonmember attempt",
			AuthHeader:   testutil.GetAuthHeader(nonmember.Token),
			GivenUserID:  member.ID,
			ExpectStatus: http.StatusNotFound,
		},
		{
			Name:         "member attempt to remove other member",
			AuthHeader:   testutil.GetAuthHeader(member.Token),
			GivenUserID:  memberToRemove.ID,
			ExpectStatus: http.StatusNotFound,
		},
		{
			Name:         "bad auth header",
			AuthHeader:   map[string]string{"boop": "beep"},
			GivenUserID:  member.ID,
			ExpectStatus: http.StatusUnauthorized,
		},
		{
			Name:              "owner remove member success",
			AuthHeader:        testutil.GetAuthHeader(owner.Token),
			GivenUserID:       memberToRemove.ID,
			ExpectStatus:      http.StatusOK,
			ExpectMemberIDs:   []string{owner.ID, member.ID, memberToLeave.ID},
			ExpectMemberNames: []string{member.FullName, owner.FullName, memberToLeave.FullName},
		},
		{
			Name:              "member remove self success",
			AuthHeader:        testutil.GetAuthHeader(memberToLeave.Token),
			GivenUserID:       memberToLeave.ID,
			ExpectStatus:      http.StatusOK,
			ExpectMemberIDs:   []string{owner.ID, member.ID},
			ExpectMemberNames: []string{member.FullName, owner.FullName},
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.Name, func(t *testing.T) {
			tt := apitest.New(tcase.Name).
				Handler(_handler).
				Delete(fmt.Sprintf("/threads/%s/users/%s", thread.ID, tcase.GivenUserID)).
				JSON(`{}`).
				Headers(tcase.AuthHeader).
				Expect(t).
				Status(tcase.ExpectStatus)

			if tcase.ExpectStatus <= http.StatusBadRequest {
				tt.Assert(jsonpath.Equal("$.id", thread.ID))
				tt.Assert(jsonpath.Equal("$.owner.id", owner.ID))
				tt.Assert(jsonpath.Equal("$.owner.fullName", owner.FullName))
				for i := range tcase.ExpectMemberNames {
					tt.Assert(jsonpath.Equal(
						fmt.Sprintf("$.users[%d].fullName", i),
						tcase.ExpectMemberNames[i]))
				}
			}

			tt.End()
		})
	}
}
