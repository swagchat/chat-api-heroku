package storage

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/swagchat/chat-api/models"
	"github.com/swagchat/chat-api/utils"
)

type LocalStorageProvider struct {
	baseUrl   string
	localPath string
}

func (provider LocalStorageProvider) Init() error {
	return nil
}

func (provider LocalStorageProvider) Post(assetInfo *AssetInfo) (string, *models.ProblemDetail) {
	if err := os.MkdirAll(provider.localPath, 0775); err != nil {
		return "", &models.ProblemDetail{
			Title:     "Create directory failed. (Local Storage)",
			Status:    http.StatusInternalServerError,
			ErrorName: "storage-error",
			Detail:    err.Error(),
		}
	}

	data, err := ioutil.ReadAll(assetInfo.Data)
	if err != nil {
		return "", &models.ProblemDetail{
			Title:     "Reading asset data failed. (Local Storage)",
			Status:    http.StatusInternalServerError,
			ErrorName: "storage-error",
			Detail:    err.Error(),
		}
	}

	filePath := utils.AppendStrings(provider.localPath, "/", assetInfo.FileName)
	err = ioutil.WriteFile(filePath, data, 0644)
	if err != nil {
		return "", &models.ProblemDetail{
			Title:     "Writing asset data failed. (Local Storage)",
			Status:    http.StatusInternalServerError,
			ErrorName: "storage-error",
			Detail:    err.Error(),
		}
	}
	sourceUrl := utils.AppendStrings(provider.baseUrl, "/", assetInfo.FileName)
	log.Println("sourceUrl:", sourceUrl)
	return sourceUrl, nil
}

func (provider LocalStorageProvider) Get(assetInfo *AssetInfo) ([]byte, *models.ProblemDetail) {
	file, err := os.Open(fmt.Sprintf("%s/%s", provider.localPath, assetInfo.FileName))
	defer file.Close()
	if err != nil {
		return nil, &models.ProblemDetail{
			Title:     "Opening asset data failed. (Local Storage)",
			Status:    http.StatusInternalServerError,
			ErrorName: "storage-error",
			Detail:    err.Error(),
		}
	}

	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, &models.ProblemDetail{
			Title:     "Reading asset data failed. (Local Storage)",
			Status:    http.StatusInternalServerError,
			ErrorName: "storage-error",
			Detail:    err.Error(),
		}
	}

	return bytes, nil
}
