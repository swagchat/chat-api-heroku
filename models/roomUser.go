package models

import (
	"net/http"

	"encoding/json"
	"time"

	"github.com/swagchat/chat-api/utils"
)

type RoomUser struct {
	RoomId      string         `json:"roomId" db:"room_id,notnull"`
	UserId      string         `json:"userId" db:"user_id,notnull"`
	UnreadCount *int64         `json:"unreadCount" db:"unread_count"`
	MetaData    utils.JSONText `json:"metaData" db:"meta_data"`
	Created     int64          `json:"created" db:"created,notnull"`
	Modified    int64          `json:"modified" db:"modified,notnull"`
}

func (ru *RoomUser) MarshalJSON() ([]byte, error) {
	l, _ := time.LoadLocation("Etc/GMT")
	return json.Marshal(&struct {
		RoomId      string         `json:"roomId"`
		UserId      string         `json:"userId"`
		UnreadCount *int64         `json:"unreadCount"`
		MetaData    utils.JSONText `json:"metaData"`
		Created     string         `json:"created"`
		Modified    string         `json:"modified"`
	}{
		RoomId:      ru.RoomId,
		UserId:      ru.UserId,
		UnreadCount: ru.UnreadCount,
		MetaData:    ru.MetaData,
		Created:     time.Unix(ru.Created, 0).In(l).Format(time.RFC3339),
		Modified:    time.Unix(ru.Modified, 0).In(l).Format(time.RFC3339),
	})
}

func (ru *RoomUser) IsValid() *ProblemDetail {
	if ru.RoomId != "" && !utils.IsValidId(ru.RoomId) {
		return &ProblemDetail{
			Title:     "Request parameter error. (Create room user item)",
			Status:    http.StatusBadRequest,
			ErrorName: ERROR_NAME_INVALID_PARAM,
			InvalidParams: []InvalidParam{
				InvalidParam{
					Name:   "roomId",
					Reason: "roomId is invalid. Available characters are alphabets, numbers and hyphens.",
				},
			},
		}
	}

	if ru.UserId != "" && !utils.IsValidId(ru.UserId) {
		return &ProblemDetail{
			Title:     "Request parameter error. (Create room user item)",
			Status:    http.StatusBadRequest,
			ErrorName: ERROR_NAME_INVALID_PARAM,
			InvalidParams: []InvalidParam{
				InvalidParam{
					Name:   "userId",
					Reason: "userId is invalid. Available characters are alphabets, numbers and hyphens.",
				},
			},
		}
	}

	return nil
}

func (ru *RoomUser) BeforeSave() {
	nowTimestamp := time.Now().Unix()
	if ru.Created == 0 {
		ru.Created = nowTimestamp
	}
	ru.Modified = nowTimestamp
}

func (ru *RoomUser) Put(put *RoomUser) {
	if put.UnreadCount != nil {
		ru.UnreadCount = put.UnreadCount
	}
	if put.MetaData != nil {
		ru.MetaData = put.MetaData
	}
}

type ErrorRoomUser struct {
	UserId string         `json:"userId,omitempty"`
	Error  *ProblemDetail `json:"error"`
}

type ResponseRoomUser struct {
	RoomUsers []RoomUser      `json:"roomUsers,omitempty"`
	Errors    []ErrorRoomUser `json:"errors,omitempty"`
}

type RequestRoomUserIds struct {
	UserIds []string `json:"userIds,omitempty" db:"-"`
}

type RoomUsers struct {
	RoomUsers []*RoomUser `json:"roomUsers"`
}

func (rus *RequestRoomUserIds) IsValid(method string, r *Room) *ProblemDetail {
	if len(rus.UserIds) == 0 {
		return &ProblemDetail{
			Title:     "Request parameter error. (Create room's user list)",
			Status:    http.StatusBadRequest,
			ErrorName: ERROR_NAME_INVALID_PARAM,
			InvalidParams: []InvalidParam{
				InvalidParam{
					Name:   "userIds",
					Reason: "Not set.",
				},
			},
		}
	}

	if *r.Type == ONE_ON_ONE {
		for _, userId := range rus.UserIds {
			if userId == r.UserId {
				return &ProblemDetail{
					Title:     "Request parameter error. (Create room's user list)",
					Status:    http.StatusBadRequest,
					ErrorName: ERROR_NAME_INVALID_PARAM,
					InvalidParams: []InvalidParam{
						InvalidParam{
							Name:   "userIds",
							Reason: "In case of 1-on-1 room type, it must always set one userId different from this room's userId.",
						},
					},
				}
			}
		}
	}

	if method == "POST" && *r.Type == ONE_ON_ONE {
		if len(rus.UserIds) == 2 {
			return &ProblemDetail{
				Title:     "Request parameter error. (Create room's user list)",
				Status:    http.StatusBadRequest,
				ErrorName: ERROR_NAME_INVALID_PARAM,
				InvalidParams: []InvalidParam{
					InvalidParam{
						Name:   "userIds",
						Reason: "In case of 1-on-1 room type, It can only update once.",
					},
				},
			}
		}
	}

	if method == "PUT" && *r.Type == ONE_ON_ONE {
		if len(r.Users) == 2 {
			return &ProblemDetail{
				Title:     "Request parameter error. (Create room's user list)",
				Status:    http.StatusBadRequest,
				ErrorName: ERROR_NAME_INVALID_PARAM,
				InvalidParams: []InvalidParam{
					InvalidParam{
						Name:   "userIds",
						Reason: "In case of 1-on-1 room type, It can only update once.",
					},
				},
			}
		}
	}

	if method == "DELETE" && *r.Type == ONE_ON_ONE {
		return &ProblemDetail{
			Title:     "Operation not permitted. (Delete room's user item)",
			Status:    http.StatusBadRequest,
			ErrorName: ERROR_NAME_OPERATION_NOT_PERMITTED,
		}
	}

	return nil
}

func (rus *RequestRoomUserIds) RemoveDuplicate() {
	rus.UserIds = utils.RemoveDuplicate(rus.UserIds)
}
