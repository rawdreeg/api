package handler_test

import (
	"net/http"
	"testing"

	"github.com/icrowley/fake"
	"github.com/steinfletcher/apitest"
	jsonpath "github.com/steinfletcher/apitest-jsonpath"

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
					map[string]string{
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
					map[string]string{
						"id": u2.ID,
					},
					map[string]string{
						"email": "test@testing.com",
					},
					map[string]string{
						"email": "test@testing.com",
					},
					map[string]string{
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
					map[string]string{
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
					map[string]string{
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
