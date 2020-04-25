package models

import (
	"context"
)

func (c *Client) GetContactsByUser(ctx context.Context, u *User) ([]*UserPartial, error) {
	contacts := make([]*User, len(u.ContactKeys))
	if err := c.db.GetMulti(ctx, u.ContactKeys, contacts); err != nil {
		return []*UserPartial{}, err
	}

	return MapUsersToUserPartials(contacts), nil
}
