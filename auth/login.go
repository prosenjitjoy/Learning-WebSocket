package auth

type UserLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	OTP string `json:"otp"`
}
