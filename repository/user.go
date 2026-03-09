package repository

import (
	"context"

	"cloud.google.com/go/firestore"
)

type UserPreferences struct {
	AutoJoinCalendar bool `json:"auto_join_calendar" firestore:"autoJoinCalendar"`
}

type UserProfile struct {
	UID              string          `json:"uid" firestore:"uid"`
	Preferences      UserPreferences `json:"preferences" firestore:"preferences"`
	RecallCalendarID string          `json:"recall_calendar_id,omitempty" firestore:"recallCalendarID,omitempty"`
}

type FirestoreUserRepository struct {
	client *firestore.Client
}

func NewFirestoreUserRepository(client *firestore.Client) *FirestoreUserRepository {
	return &FirestoreUserRepository{client: client}
}

func (r *FirestoreUserRepository) GetProfile(ctx context.Context, uid string) (*UserProfile, error) {
	doc, err := r.client.Collection("users").Doc(uid).Get(ctx)
	if err != nil {
		// If not found, return default profile instead of erroring
		return &UserProfile{
			UID: uid,
			Preferences: UserPreferences{
				AutoJoinCalendar: false,
			},
		}, nil
	}

	var profile UserProfile
	if err := doc.DataTo(&profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

func (r *FirestoreUserRepository) UpdatePreferences(ctx context.Context, uid string, prefs UserPreferences) error {
	_, err := r.client.Collection("users").Doc(uid).Set(ctx, map[string]interface{}{
		"uid": uid,
		"preferences": map[string]interface{}{
			"autoJoinCalendar": prefs.AutoJoinCalendar,
		},
	}, firestore.MergeAll)
	return err
}

func (r *FirestoreUserRepository) SaveRecallCalendarID(ctx context.Context, uid string, calendarID string) error {
	_, err := r.client.Collection("users").Doc(uid).Set(ctx, map[string]interface{}{
		"uid":              uid,
		"recallCalendarID": calendarID,
	}, firestore.MergeAll)
	return err
}
