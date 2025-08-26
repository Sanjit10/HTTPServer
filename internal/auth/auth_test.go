package auth

import (
	"net/http"
	"testing"
)

func TestGetBearereToken(t *testing.T) {
	tests := []struct {
        name       string
        header     http.Header
        wantToken  string
        wantErr    bool
        errMessage string
    }{
        {
            name:       "missing header",
            header:     http.Header{},
            wantToken:  "",
            wantErr:    true,
            errMessage: "authorization header missing",
        },
        {
            name: "valid bearer token",
            header: http.Header{
                "Authorization": []string{"Bearer abc123"},
            },
            wantToken: "abc123",
            wantErr:   false,
        },
        {
            name: "invalid prefix",
            header: http.Header{
                "Authorization": []string{"Token abc123"},
            },
            wantToken:  "",
            wantErr:    true,
            errMessage: "invalid authorization header format",
        },
        {
            name: "empty token",
            header: http.Header{
                "Authorization": []string{"Bearer "},
            },
            wantToken:  "",
            wantErr:    true,
            errMessage: "empty bearer token",
        },
    }
	for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := GetBearerToken(tt.header)

            if (err != nil) != tt.wantErr {
                t.Fatalf("expected error=%v, got %v", tt.wantErr, err)
            }

            if got != tt.wantToken {
                t.Errorf("expected token=%q, got %q", tt.wantToken, got)
            }

            if tt.wantErr && err != nil && tt.errMessage != "" {
                if err.Error() != tt.errMessage {
                    t.Errorf("expected error message=%q, got %q", tt.errMessage, err.Error())
                }
            }
        })
    }
}
