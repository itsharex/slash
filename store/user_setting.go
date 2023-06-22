package store

import (
	"context"
	"database/sql"
	"strings"
)

type UserSetting struct {
	UserID int
	Key    string
	Value  string
}

type FindUserSetting struct {
	UserID *int
	Key    string
}

func (s *Store) UpsertUserSetting(ctx context.Context, upsert *UserSetting) (*UserSetting, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	query := `
		INSERT INTO user_setting (
			user_id, key, value
		)
		VALUES (?, ?, ?)
		ON CONFLICT(user_id, key) DO UPDATE 
		SET value = EXCLUDED.value
	`
	if _, err := tx.ExecContext(ctx, query, upsert.UserID, upsert.Key, upsert.Value); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	userSettingMessage := upsert
	s.userSettingCache.Store(getUserSettingCacheKey(userSettingMessage.UserID, userSettingMessage.Key), userSettingMessage)
	return userSettingMessage, nil
}

func (s *Store) ListUserSettings(ctx context.Context, find *FindUserSetting) ([]*UserSetting, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	userSettingList, err := listUserSettings(ctx, tx, find)
	if err != nil {
		return nil, err
	}

	for _, userSetting := range userSettingList {
		s.userSettingCache.Store(getUserSettingCacheKey(userSetting.UserID, userSetting.Key), userSetting)
	}
	return userSettingList, nil
}

func (s *Store) GetUserSetting(ctx context.Context, find *FindUserSetting) (*UserSetting, error) {
	if find.UserID != nil && find.Key != "" {
		if cache, ok := s.userSettingCache.Load(getUserSettingCacheKey(*find.UserID, find.Key)); ok {
			return cache.(*UserSetting), nil
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	list, err := listUserSettings(ctx, tx, find)
	if err != nil {
		return nil, err
	}

	if len(list) == 0 {
		return nil, nil
	}

	userSettingMessage := list[0]
	s.userSettingCache.Store(getUserSettingCacheKey(userSettingMessage.UserID, userSettingMessage.Key), userSettingMessage)
	return userSettingMessage, nil
}

func listUserSettings(ctx context.Context, tx *sql.Tx, find *FindUserSetting) ([]*UserSetting, error) {
	where, args := []string{"1 = 1"}, []any{}

	if v := find.Key; v != "" {
		where, args = append(where, "key = ?"), append(args, v)
	}
	if v := find.UserID; v != nil {
		where, args = append(where, "user_id = ?"), append(args, *find.UserID)
	}

	query := `
		SELECT
			user_id,
		  key,
			value
		FROM user_setting
		WHERE ` + strings.Join(where, " AND ")
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	userSettingMessageList := make([]*UserSetting, 0)
	for rows.Next() {
		userSettingMessage := &UserSetting{}
		if err := rows.Scan(
			&userSettingMessage.UserID,
			&userSettingMessage.Key,
			&userSettingMessage.Value,
		); err != nil {
			return nil, err
		}
		userSettingMessageList = append(userSettingMessageList, userSettingMessage)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return userSettingMessageList, nil
}
