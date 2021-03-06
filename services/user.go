// Business Logic

package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"go.uber.org/zap"

	"github.com/swagchat/chat-api/datastore"
	"github.com/swagchat/chat-api/models"
	"github.com/swagchat/chat-api/notification"
	"github.com/swagchat/chat-api/utils"
)

func PostUser(post *models.User) (*models.User, *models.ProblemDetail) {
	if pd := post.IsValid(); pd != nil {
		return nil, pd
	}

	post.BeforeSave()
	dRes := datastore.GetProvider().InsertUser(post)
	if dRes.ProblemDetail != nil {
		return nil, dRes.ProblemDetail
	}
	return dRes.Data.(*models.User), nil
}

func GetUsers() (*models.Users, *models.ProblemDetail) {
	dRes := datastore.GetProvider().SelectUsers()
	if dRes.ProblemDetail != nil {
		return nil, dRes.ProblemDetail
	}

	users := &models.Users{
		Users: dRes.Data.([]*models.User),
	}
	return users, nil
}

func GetUser(userId string) (*models.User, *models.ProblemDetail) {
	dRes := datastore.GetProvider().SelectUser(userId, true, true, true)
	if dRes.ProblemDetail != nil {
		return nil, dRes.ProblemDetail
	}
	if dRes.Data == nil {
		return nil, &models.ProblemDetail{
			Status: http.StatusNotFound,
		}
	}

	user := dRes.Data.(*models.User)
	unreadCountRooms := make([]*models.RoomForUser, 0)
	notUnreadCountRooms := make([]*models.RoomForUser, 0)
	for _, roomForUser := range user.Rooms {
		if roomForUser.RuUnreadCount > 0 {
			unreadCountRooms = append(unreadCountRooms, roomForUser)
		} else {
			notUnreadCountRooms = append(notUnreadCountRooms, roomForUser)
		}
	}
	mergeRooms := append(unreadCountRooms, notUnreadCountRooms...)
	user.Rooms = mergeRooms
	user.AccessToken = ""
	return user, nil
}

func PutUser(put *models.User) (*models.User, *models.ProblemDetail) {
	user, pd := selectUser(put.UserId)
	if pd != nil {
		return nil, pd
	}

	user.Put(put)
	if pd := user.IsValid(); pd != nil {
		return nil, pd
	}
	user.BeforeSave()
	dRes := datastore.GetProvider().UpdateUser(user)
	if dRes.ProblemDetail != nil {
		return nil, dRes.ProblemDetail
	}

	user.AccessToken = ""
	return user, nil
}

func DeleteUser(userId string) *models.ProblemDetail {
	// User existence check
	_, pd := selectUser(userId)
	if pd != nil {
		return pd
	}

	dRes := datastore.GetProvider().SelectDevicesByUserId(userId)
	if dRes.ProblemDetail != nil {
		return dRes.ProblemDetail
	}
	if dRes.Data != nil {
		devices := dRes.Data.([]*models.Device)
		for _, device := range devices {
			nRes := <-notification.GetProvider().DeleteEndpoint(device.NotificationDeviceId)
			if nRes.ProblemDetail != nil {
				return nRes.ProblemDetail
			}
		}
	}

	dRes = datastore.GetProvider().UpdateUserDeleted(userId)
	if dRes.ProblemDetail != nil {
		return dRes.ProblemDetail
	}

	ctx, _ := context.WithCancel(context.Background())
	go unsubscribeByUserId(ctx, userId)

	return nil
}

func selectUser(userId string) (*models.User, *models.ProblemDetail) {
	dRes := datastore.GetProvider().SelectUser(userId, false, false, false)
	if dRes.ProblemDetail != nil {
		return nil, dRes.ProblemDetail
	}
	if dRes.Data == nil {
		return nil, &models.ProblemDetail{
			Status: http.StatusNotFound,
		}
	}
	return dRes.Data.(*models.User), nil
}

func unsubscribeByUserId(ctx context.Context, userId string) {
	dRes := datastore.GetProvider().SelectDeletedSubscriptionsByUserId(userId)
	if dRes.ProblemDetail != nil {
		pdBytes, _ := json.Marshal(dRes.ProblemDetail)
		utils.AppLogger.Error("",
			zap.String("problemDetail", string(pdBytes)),
			zap.String("err", fmt.Sprintf("%+v", dRes.ProblemDetail.Error)),
		)
	}
	unsubscribe(ctx, dRes.Data.([]*models.Subscription))
}

func GetUserUnreadCount(userId string) (*models.UserUnreadCount, *models.ProblemDetail) {
	user, pd := selectUser(userId)
	if pd != nil {
		return nil, pd
	}

	userUnreadCount := &models.UserUnreadCount{
		UnreadCount: user.UnreadCount,
	}
	return userUnreadCount, nil
}
