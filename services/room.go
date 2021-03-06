package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/swagchat/chat-api/datastore"
	"github.com/swagchat/chat-api/models"
	"github.com/swagchat/chat-api/notification"
	"github.com/swagchat/chat-api/utils"
	"go.uber.org/zap"
)

func PostRoom(post *models.Room) (*models.Room, *models.ProblemDetail) {
	if pd := post.IsValid(); pd != nil {
		return nil, pd
	}
	post.BeforeSave()
	post.RequestRoomUserIds.RemoveDuplicate()

	if *post.Type == models.ONE_ON_ONE {
		dRes := datastore.GetProvider().SelectRoomUserOfOneOnOne(post.UserId, post.RequestRoomUserIds.UserIds[0])
		if dRes.ProblemDetail != nil {
			return nil, dRes.ProblemDetail
		}
		if dRes.Data != nil {
			return nil, &models.ProblemDetail{
				Status: http.StatusConflict,
			}
		}
	}

	if pd := post.RequestRoomUserIds.IsValid("POST", post); pd != nil {
		return nil, pd
	}

	if post.RequestRoomUserIds.UserIds != nil {
		notificationTopicId, pd := createTopic(post.RoomId)
		if pd != nil {
			return nil, pd
		}
		post.NotificationTopicId = notificationTopicId
	}

	dRes := datastore.GetProvider().InsertRoom(post)
	if dRes.ProblemDetail != nil {
		return nil, dRes.ProblemDetail
	}
	room := dRes.Data.(*models.Room)

	dRes = datastore.GetProvider().SelectUsersForRoom(room.RoomId)
	if dRes.ProblemDetail != nil {
		return nil, dRes.ProblemDetail
	}
	room.Users = dRes.Data.([]*models.UserForRoom)

	dRes = datastore.GetProvider().SelectRoomUsersByRoomId(room.RoomId)
	if dRes.ProblemDetail != nil {
		return nil, dRes.ProblemDetail
	}
	roomUsers := dRes.Data.([]*models.RoomUser)

	ctx, _ := context.WithCancel(context.Background())
	go subscribeByRoomUsers(ctx, roomUsers)
	go publishUserJoin(room.RoomId)

	return room, nil
}

func GetRooms(values url.Values) (*models.Rooms, *models.ProblemDetail) {
	dRes := datastore.GetProvider().SelectRooms()
	if dRes.ProblemDetail != nil {
		return nil, dRes.ProblemDetail
	}

	rooms := &models.Rooms{
		Rooms: dRes.Data.([]*models.Room),
	}
	dRes = datastore.GetProvider().SelectCountRooms()
	rooms.AllCount = dRes.Data.(int64)
	return rooms, nil
}

func GetRoom(roomId string) (*models.Room, *models.ProblemDetail) {
	room, pd := selectRoom(roomId)
	if pd != nil {
		return nil, pd
	}

	dRes := datastore.GetProvider().SelectUsersForRoom(roomId)
	if dRes.ProblemDetail != nil {
		return nil, dRes.ProblemDetail
	}
	room.Users = dRes.Data.([]*models.UserForRoom)

	dRes = datastore.GetProvider().SelectCountMessagesByRoomId(roomId)
	if dRes.ProblemDetail != nil {
		return nil, dRes.ProblemDetail
	}
	room.MessageCount = dRes.Data.(int64)
	return room, nil
}

func PutRoom(put *models.Room) (*models.Room, *models.ProblemDetail) {
	room, pd := selectRoom(put.RoomId)
	if pd != nil {
		return nil, pd
	}

	if pd := room.Put(put); pd != nil {
		return nil, pd
	}

	if pd := room.IsValid(); pd != nil {
		return nil, pd
	}
	room.BeforeSave()

	dRes := datastore.GetProvider().UpdateRoom(room)
	if dRes.ProblemDetail != nil {
		return nil, dRes.ProblemDetail
	}
	room = dRes.Data.(*models.Room)

	dRes = datastore.GetProvider().SelectUsersForRoom(room.RoomId)
	if dRes.ProblemDetail != nil {
		return nil, dRes.ProblemDetail
	}
	room.Users = dRes.Data.([]*models.UserForRoom)
	return room, nil
}

func DeleteRoom(roomId string) *models.ProblemDetail {
	room, pd := selectRoom(roomId)
	if pd != nil {
		return pd
	}

	if room.NotificationTopicId != "" {
		nRes := <-notification.GetProvider().DeleteTopic(room.NotificationTopicId)
		if nRes.ProblemDetail != nil {
			return nRes.ProblemDetail
		}
	}

	room.Deleted = time.Now().Unix()
	dRes := datastore.GetProvider().UpdateRoomDeleted(roomId)
	if dRes.ProblemDetail != nil {
		return dRes.ProblemDetail
	}

	ctx, _ := context.WithCancel(context.Background())
	go func() {
		wg := &sync.WaitGroup{}
		wg.Add(1)
		go unsubscribeByRoomId(ctx, roomId, wg)
		wg.Wait()
		room.NotificationTopicId = ""
		datastore.GetProvider().UpdateRoom(room)
	}()

	return nil
}

func GetRoomMessages(roomId string, params url.Values) (*models.Messages, *models.ProblemDetail) {
	limit, offset, order, pd := setPagingParams(params)
	if pd != nil {
		return nil, pd
	}

	dRes := datastore.GetProvider().SelectMessages(roomId, limit, offset, order)
	if dRes.ProblemDetail != nil {
		return nil, dRes.ProblemDetail
	}
	messages := &models.Messages{
		Messages: dRes.Data.([]*models.Message),
	}

	dRes = datastore.GetProvider().SelectCountMessagesByRoomId(roomId)
	if dRes.ProblemDetail != nil {
		return nil, dRes.ProblemDetail
	}
	messages.AllCount = dRes.Data.(int64)
	return messages, nil
}

func selectRoom(roomId string) (*models.Room, *models.ProblemDetail) {
	dRes := datastore.GetProvider().SelectRoom(roomId)
	if dRes.ProblemDetail != nil {
		return nil, dRes.ProblemDetail
	}
	if dRes.Data == nil {
		return nil, &models.ProblemDetail{
			Status: http.StatusNotFound,
		}
	}
	return dRes.Data.(*models.Room), nil
}

func unsubscribeByRoomId(ctx context.Context, roomId string, wg *sync.WaitGroup) {
	dRes := datastore.GetProvider().SelectDeletedSubscriptionsByRoomId(roomId)
	if dRes.ProblemDetail != nil {
		pdBytes, _ := json.Marshal(dRes.ProblemDetail)
		utils.AppLogger.Error("",
			zap.String("problemDetail", string(pdBytes)),
			zap.String("err", fmt.Sprintf("%+v", dRes.ProblemDetail.Error)),
		)
	}
	<-unsubscribe(ctx, dRes.Data.([]*models.Subscription))
	if wg != nil {
		wg.Done()
	}
}

func setPagingParams(params url.Values) (int, int, string, *models.ProblemDetail) {
	var err error
	limit := 10
	offset := 0
	order := "ASC"
	if limitArray, ok := params["limit"]; ok {
		limit, err = strconv.Atoi(limitArray[0])
		if err != nil {
			return limit, offset, order, &models.ProblemDetail{
				Title:     "Request parameter error.",
				Status:    http.StatusBadRequest,
				ErrorName: models.ERROR_NAME_INVALID_PARAM,
				InvalidParams: []models.InvalidParam{
					models.InvalidParam{
						Name:   "limit",
						Reason: "limit is incorrect.",
					},
				},
			}
		}
	}
	if offsetArray, ok := params["offset"]; ok {
		offset, err = strconv.Atoi(offsetArray[0])
		if err != nil {
			return limit, offset, order, &models.ProblemDetail{
				Title:     "Request parameter error.",
				Status:    http.StatusBadRequest,
				ErrorName: models.ERROR_NAME_INVALID_PARAM,
				InvalidParams: []models.InvalidParam{
					models.InvalidParam{
						Name:   "offset",
						Reason: "offset is incorrect.",
					},
				},
			}
		}
	}
	if orderArray, ok := params["order"]; ok {
		order := orderArray[0]
		allowedOrders := []string{
			"DESC",
			"desc",
			"ASC",
			"asc",
		}
		if utils.SearchStringValueInSlice(allowedOrders, order) {
			return limit, offset, order, &models.ProblemDetail{
				Title:     "Request parameter error.",
				Status:    http.StatusBadRequest,
				ErrorName: models.ERROR_NAME_INVALID_PARAM,
				InvalidParams: []models.InvalidParam{
					models.InvalidParam{
						Name:   "order",
						Reason: "order is incorrect.",
					},
				},
			}
		}
	}
	return limit, offset, order, nil
}
