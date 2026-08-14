package model

import "errors"

func searchUsersByIPv4(ip string, group string, role *int, status *int, startIdx int, num int, sortOptions ...UserSortOptions) ([]*User, int64, error) {
	if LOG_DB == nil {
		return nil, 0, errors.New("log database is not initialized")
	}
	var userIDs []int
	if err := LOG_DB.Model(&Log{}).
		Where("user_id > ? AND ip = ?", 0, ip).
		Distinct("user_id").
		Pluck("user_id", &userIDs).Error; err != nil {
		return nil, 0, err
	}
	if len(userIDs) == 0 {
		return []*User{}, 0, nil
	}
	query := DB.Unscoped().Model(&User{}).Where("id IN ?", userIDs)
	if group != "" {
		query = query.Where(commonGroupCol+" = ?", group)
	}
	if role != nil {
		query = query.Where("role = ?", *role)
	}
	if status != nil {
		if *status == -1 {
			query = query.Where("deleted_at IS NOT NULL")
		} else {
			query = query.Where("deleted_at IS NULL").Where("status = ?", *status)
		}
	} else {
		query = query.Where("deleted_at IS NULL")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var users []*User
	order := resolveUserSortOptions(sortOptions)
	if err := order.Apply(query.Omit("password", "access_token")).Limit(num).Offset(startIdx).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}
