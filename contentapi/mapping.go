package contentapi

// Map users into a map for faster lookup (especially repeated)
func GetMappedUsers(users []User) map[int64]*User {
	result := make(map[int64]*User)
	for i := range users {
		result[users[i].Id] = &users[i]
	}
	return result
}

// Attempt to apply a user to the given content
func (c *Content) ApplyUser(users map[int64]*User) *User {
	user, ok := users[c.CreateUserId]
	if !ok {
		return nil
	}
	c.CreateUser = user
	return user
}

// Attempt to apply a user to the given comment
func (c *Comment) ApplyUser(users map[int64]*User) *User {
	user, ok := users[c.CreateUserId]
	if !ok {
		return nil
	}
	c.CreateUser = user
	return user
}
