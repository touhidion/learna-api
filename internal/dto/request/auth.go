// Package request holds the inbound payload shapes. Keeping them separate
// from models means a client can never set a field it has no business setting
// (role, is_active, created_by) simply by including it in the JSON.
package request

// Password rules are expressed with the binding tags so the validator produces
// the field-level errors; 8 is the floor and 72 the bcrypt ceiling.

type Signup struct {
	Name     string `json:"name" binding:"required,min=2,max=120"`
	Email    string `json:"email" binding:"required,email,max=255"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

type Login struct {
	Email    string `json:"email" binding:"required,email,max=255"`
	Password string `json:"password" binding:"required,min=1,max=72"`
}

type RefreshToken struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type ForgotPassword struct {
	Email string `json:"email" binding:"required,email,max=255"`
}

type ResetPassword struct {
	Token           string `json:"token" binding:"required"`
	Password        string `json:"password" binding:"required,min=8,max=72"`
	ConfirmPassword string `json:"confirm_password" binding:"required,eqfield=Password"`
}

type ChangePassword struct {
	CurrentPassword string `json:"current_password" binding:"required,min=1,max=72"`
	NewPassword     string `json:"new_password" binding:"required,min=8,max=72"`
}

type UpdateProfile struct {
	Name      *string `json:"name" binding:"omitempty,min=2,max=120"`
	AvatarURL *string `json:"avatar_url" binding:"omitempty,url,max=1024"`
}
