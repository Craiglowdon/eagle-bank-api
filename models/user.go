package models

import "time"

type Address struct {
	Line1    string `json:"line1"`
	Line2    string `json:"line2,omitempty"`
	Line3    string `json:"line3,omitempty"`
	Town     string `json:"town"`
	County   string `json:"county"`
	Postcode string `json:"postcode"`
}

type CreateUserRequest struct {
	Name        string  `json:"name"`
	Address     Address `json:"address"`
	PhoneNumber string  `json:"phoneNumber"`
	Email       string  `json:"email"`
	Password    string  `json:"password"`
}

type User struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Address          Address   `json:"address"`
	PhoneNumber      string    `json:"phoneNumber"`
	Email            string    `json:"email"`
	CreatedTimestamp time.Time `json:"createdTimestamp"`
	UpdatedTimestamp time.Time `json:"updatedTimestamp"`
}

type UpdateUserRequest struct {
	Name        *string  `json:"name"`
	Address     *Address `json:"address"`
	PhoneNumber *string  `json:"phoneNumber"`
	Email       *string  `json:"email"`
}
