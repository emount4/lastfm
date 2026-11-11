package registration

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/emount4/lastfm/internal/domain"
)

type registrationStub struct {
	profile     string
	found       bool
	lookupErr   error
	scrobbleErr error
	saveErr     error
	deleteErr   error
	calls       []string
	saved       domain.Participant
	deletedID   int64
}

func (s *registrationStub) LookupProfile(_ context.Context, username string) (string, bool, error) {
	s.calls = append(s.calls, "lookup "+username)
	return s.profile, s.found, s.lookupErr
}

func (s *registrationStub) CheckScrobbleAccess(_ context.Context, username string) error {
	s.calls = append(s.calls, "scrobbles "+username)
	return s.scrobbleErr
}

func (s *registrationStub) Save(_ context.Context, participant domain.Participant) error {
	s.calls = append(s.calls, "save")
	s.saved = participant
	return s.saveErr
}

func (s *registrationStub) Delete(_ context.Context, userID int64) error {
	s.deletedID = userID
	return s.deleteErr
}

func TestUnlink(t *testing.T) {
	storeErr := errors.New("database unavailable")
	stub := &registrationStub{deleteErr: storeErr}
	service := NewService(nil, stub)
	if err := service.Unlink(t.Context(), -1); err == nil || stub.deletedID != 0 {
		t.Fatalf("invalid ID: error = %v, deleted ID = %d", err, stub.deletedID)
	}
	if err := service.Unlink(t.Context(), 123); !errors.Is(err, storeErr) || stub.deletedID != 123 {
		t.Fatalf("error = %v, deleted ID = %d", err, stub.deletedID)
	}
}

func TestRegister(t *testing.T) {
	upstreamErr := errors.New("Last.fm unavailable")
	accessErr := errors.New("scrobbles unavailable")
	tests := []struct {
		name      string
		username  string
		stub      registrationStub
		wantCalls []string
		wantErr   error
	}{
		{
			name:      "canonical nickname is saved",
			username:  "  example  ",
			stub:      registrationStub{profile: "Example", found: true},
			wantCalls: []string{"lookup example", "scrobbles Example", "save"},
		},
		{
			name:     "empty nickname",
			username: "  ",
			wantErr:  ErrUsernameRequired,
		},
		{
			name:      "profile not found",
			username:  "example",
			wantCalls: []string{"lookup example"},
			wantErr:   ErrProfileNotFound,
		},
		{
			name:      "profile lookup failed",
			username:  "example",
			stub:      registrationStub{lookupErr: upstreamErr},
			wantCalls: []string{"lookup example"},
			wantErr:   upstreamErr,
		},
		{
			name:      "scrobbles unavailable",
			username:  "example",
			stub:      registrationStub{profile: "Example", found: true, scrobbleErr: accessErr},
			wantCalls: []string{"lookup example", "scrobbles Example"},
			wantErr:   accessErr,
		},
		{
			name:      "account already linked",
			username:  "example",
			stub:      registrationStub{profile: "Example", found: true, saveErr: domain.ErrLastFMAccountTaken},
			wantCalls: []string{"lookup example", "scrobbles Example", "save"},
			wantErr:   domain.ErrLastFMAccountTaken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := tt.stub
			service := NewService(&stub, &stub)
			participant := domain.Participant{
				TelegramUserID:      123456789,
				TelegramUsername:    "telegram_user",
				TelegramDisplayName: "Имя пользователя",
				LastFMUsername:      tt.username,
			}

			name, err := service.Register(t.Context(), participant)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got error %v, want %v", err, tt.wantErr)
			}
			if !slices.Equal(stub.calls, tt.wantCalls) {
				t.Fatalf("got calls %v, want %v", stub.calls, tt.wantCalls)
			}
			if tt.wantErr != nil {
				if name != "" {
					t.Fatalf("got name %q on failed registration", name)
				}
				return
			}

			participant.LastFMUsername = "Example"
			if name != "Example" || stub.saved != participant {
				t.Fatalf("got name %q and saved participant %+v, want %+v", name, stub.saved, participant)
			}
		})
	}
}
