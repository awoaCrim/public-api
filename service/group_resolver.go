package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/model"
)

// ResolveGroupSelection resolves the effective original groups and the
// request's final group. A fixed token is one group; auto uses the complete
// effective group set when channel selection expands it.
var (
	ErrRoutingGroupNotGranted = errors.New("routing_group_not_granted")
	ErrRoutingGroupInvalid    = errors.New("routing_group_invalid")
	ErrRoutingAutoNotAllowed  = errors.New("auto_routing_group_not_allowed")
)

// GroupSelection carries the resolved routing identity of one request.
type GroupSelection struct {
	UserID          int
	UserGroup       string
	EffectiveGroups map[string]string
	AutoGroups      []string
	TokenGroup      string
	RequestedGroup  string
	UsingGroup      string
}

// ResolveGroupSelection validates a token group against the user's current
// effective permission set and resolves the request's final group. An empty
// token group falls back to the account tier. A fixed token can only request
// that same group; an auto token may request any effective fixed group (auto
// is never a valid requested group). Root users bypass grant checks, but a
// fixed token remains fixed.
func ResolveGroupSelection(userID int, tokenGroup, requestedGroup string) (*GroupSelection, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("user id is required")
	}
	user, err := model.GetUserById(userID, false)
	if err != nil {
		return nil, err
	}
	access, err := ResolveUserGroupAccess(model.DB, userID, user.Group)
	if err != nil {
		return nil, err
	}
	selection := &GroupSelection{
		UserID:          userID,
		UserGroup:       user.Group,
		EffectiveGroups: access.Groups,
		AutoGroups:      GetAutoGroupsForUser(access.Groups),
		TokenGroup:      strings.TrimSpace(tokenGroup),
		RequestedGroup:  strings.TrimSpace(requestedGroup),
	}
	if selection.TokenGroup == "" {
		selection.TokenGroup = strings.TrimSpace(user.Group)
	}
	if selection.TokenGroup == "" {
		return nil, ErrRoutingGroupNotGranted
	}
	if selection.TokenGroup != "auto" {
		if err := validateEffectiveGroupSelection(userID, selection.EffectiveGroups, selection.TokenGroup); err != nil {
			return nil, err
		}
	}
	if selection.RequestedGroup != "" {
		if selection.RequestedGroup == "auto" {
			return nil, ErrRoutingGroupInvalid
		}
		if selection.TokenGroup != "auto" && selection.RequestedGroup != selection.TokenGroup {
			return nil, ErrRoutingGroupNotGranted
		}
		if err := validateEffectiveGroupSelection(userID, selection.EffectiveGroups, selection.RequestedGroup); err != nil {
			return nil, err
		}
		selection.UsingGroup = selection.RequestedGroup
	} else {
		selection.UsingGroup = selection.TokenGroup
	}
	return selection, nil
}

// ResolveTokenGroup returns the validated fixed token group.
func ResolveTokenGroup(userID int, tokenGroup string) (string, error) {
	selection, err := ResolveGroupSelection(userID, tokenGroup, "")
	if err != nil {
		return "", err
	}
	return selection.TokenGroup, nil
}

// ValidateRequestedGroup checks an explicit per-request group override.
func ValidateRequestedGroup(userID int, requestedGroup string) error {
	if strings.TrimSpace(requestedGroup) == "" || strings.TrimSpace(requestedGroup) == "auto" {
		return ErrRoutingGroupInvalid
	}
	_, err := ResolveGroupSelection(userID, "auto", requestedGroup)
	return err
}

func validateEffectiveGroupSelection(userID int, effective map[string]string, group string) error {
	if model.IsRootUser(userID) {
		return nil
	}
	if _, ok := effective[group]; !ok {
		return ErrRoutingGroupNotGranted
	}
	return nil
}
