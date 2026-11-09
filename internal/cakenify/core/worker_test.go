package core

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"cakenify/internal/cakenify"
	mocknotify "cakenify/internal/cakenify/mock"
	"cakenify/pkg/db/cakenifydb"
	mockdb "cakenify/pkg/db/cakenifydb/mock"
)

func mustLocation(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Africa/Johannesburg")
	require.NoError(t, err)
	return loc
}

func TestWorker_Tick_OutsideWindow(t *testing.T) {
	ctrl := gomock.NewController(t)
	db := mockdb.NewMockDB(ctrl) // no EXPECT calls set — any call fails the test
	loc := mustLocation(t)

	w := NewWorker(db, cakenify.NotifierRegistry{}, loc)
	w.now = func() time.Time { return time.Date(2024, 1, 7, 9, 0, 0, 0, loc) } // 09:00, not 10:00

	w.Tick(context.Background())
}

func TestWorker_Tick_NonSunday_DailyOnly(t *testing.T) {
	ctrl := gomock.NewController(t)
	db := mockdb.NewMockDB(ctrl)
	notifier := mocknotify.NewMockNotifier(ctrl)
	notifier.EXPECT().Strategy().Return("ntfy").AnyTimes()
	loc := mustLocation(t)

	owner := uuid.New()

	// 2024-01-08 is a Monday.
	db.EXPECT().SchedulerLastRunDate(gomock.Any(), jobDailyReminder).Return(nil, nil)
	db.EXPECT().BirthdaysForMonthDays(gomock.Any(), []cakenifydb.MonthDay{{Month: 1, Day: 8}}).
		Return([]cakenifydb.Birthday{{UserID: owner, PersonName: "David Agar"}}, nil)
	db.EXPECT().EnabledNotificationSettingsByUser(gomock.Any(), owner).
		Return([]cakenifydb.NotificationSetting{{Strategy: "ntfy", Config: []byte(`{}`), Enabled: true}}, nil)
	notifier.EXPECT().Send(gomock.Any(), gomock.Any(), cakenify.Message{Title: "Birthday today", Body: "David Agar"}).Return(nil)
	db.EXPECT().SetSchedulerLastRunDate(gomock.Any(), jobDailyReminder, gomock.Any()).Return(nil)

	w := NewWorker(db, cakenify.NotifierRegistry{"ntfy": notifier}, loc)
	w.now = func() time.Time { return time.Date(2024, 1, 8, 10, 0, 0, 0, loc) }

	w.Tick(context.Background())
}

func TestWorker_Tick_Sunday_RunsDailyAndWeekly(t *testing.T) {
	ctrl := gomock.NewController(t)
	db := mockdb.NewMockDB(ctrl)
	notifier := mocknotify.NewMockNotifier(ctrl)
	loc := mustLocation(t)

	owner := uuid.New()

	// 2024-01-07 is a Sunday. No birthday today, but one on Tuesday 2024-01-09.
	db.EXPECT().SchedulerLastRunDate(gomock.Any(), jobDailyReminder).Return(nil, nil)
	db.EXPECT().BirthdaysForMonthDays(gomock.Any(), []cakenifydb.MonthDay{{Month: 1, Day: 7}}).Return(nil, nil)

	weekDays := []cakenifydb.MonthDay{
		{Month: 1, Day: 8}, {Month: 1, Day: 9}, {Month: 1, Day: 10}, {Month: 1, Day: 11},
		{Month: 1, Day: 12}, {Month: 1, Day: 13}, {Month: 1, Day: 14},
	}
	db.EXPECT().SchedulerLastRunDate(gomock.Any(), jobWeeklyDigest).Return(nil, nil)
	db.EXPECT().BirthdaysForMonthDays(gomock.Any(), weekDays).
		Return([]cakenifydb.Birthday{{UserID: owner, PersonName: "David Agar", BirthMonth: 1, BirthDay: 9}}, nil)

	db.EXPECT().EnabledNotificationSettingsByUser(gomock.Any(), owner).
		Return([]cakenifydb.NotificationSetting{{Strategy: "ntfy", Config: []byte(`{}`), Enabled: true}}, nil).Times(1)
	notifier.EXPECT().Send(gomock.Any(), gomock.Any(), cakenify.Message{Title: "Birthdays this week", Body: "Tuesday: David Agar."}).Return(nil)

	db.EXPECT().SetSchedulerLastRunDate(gomock.Any(), jobDailyReminder, gomock.Any()).Return(nil)
	db.EXPECT().SetSchedulerLastRunDate(gomock.Any(), jobWeeklyDigest, gomock.Any()).Return(nil)

	w := NewWorker(db, cakenify.NotifierRegistry{"ntfy": notifier}, loc)
	w.now = func() time.Time { return time.Date(2024, 1, 7, 10, 0, 0, 0, loc) }

	w.Tick(context.Background())
}

func TestWorker_Tick_AlreadyRanToday_Skips(t *testing.T) {
	ctrl := gomock.NewController(t)
	db := mockdb.NewMockDB(ctrl)
	loc := mustLocation(t)

	now := time.Date(2024, 1, 8, 10, 0, 0, 0, loc)
	today := time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC)

	db.EXPECT().SchedulerLastRunDate(gomock.Any(), jobDailyReminder).Return(&today, nil)
	// No BirthdaysForMonthDays / SetSchedulerLastRunDate expected — the mock fails if they're called.

	w := NewWorker(db, cakenify.NotifierRegistry{}, loc)
	w.now = func() time.Time { return now }

	w.Tick(context.Background())
}

func TestBuildWeeklyDigestBody(t *testing.T) {
	body := buildWeeklyDigestBody([]weekdayBirthdays{
		{Weekday: "Tuesday", Names: []string{"David Agar"}},
		{Weekday: "Friday", Names: []string{"Jane Doe", "Sam Peters"}},
	})

	require.Equal(t, "Tuesday: David Agar.\nFriday: Jane Doe, Sam Peters.", body)
}
