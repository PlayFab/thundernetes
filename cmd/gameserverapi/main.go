package main

import (
	"context"
	"encoding/json"

	"net/http"

	"github.com/gin-gonic/gin"
	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	kubeClient client.Client
	ctx        = context.Background()
)

const (
	namespace                 = "default"
	buildNameParam            = "buildName"
	gameServerNameParam       = "gameServerName"
	gameServerDetailNameParam = "gameServerDetailName"
)

func main() {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = mpsv1alpha1.AddToScheme(scheme)

	var err error
	kubeClient, err = client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		log.Fatal(err)
	}

	r := setupRouter()
	// By default it serves on :8080 unless a
	// PORT environment variable was defined.
	r.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}

func setupRouter() *gin.Engine {
	r := gin.Default()
	r.POST("/gameserverbuilds", createGameServerBuild)
	r.GET("/gameserverbuilds", listGameServeBuilds)
	r.GET("/gameserverbuilds/:buildName", getGameServerBuild)
	r.DELETE("/gameserverbuilds/:buildName", deleteGameServerBuild)
	r.GET("/gameserverbuilds/:buildName/gameservers", listGameServersForBuild)
	r.GET("/gameservers/:gameServerName", getGameServer)
	r.DELETE("/gameservers/:gameServerName", deleteGameServer)
	r.PATCH("/gameserverbuilds/:buildName", patchGameServerBuild)
	r.GET("/gameserverbuilds/:buildName/gameserverdetails", listGameServerDetailsForBuild)
	r.GET("/gameserverdetails/:gameServerDetailName", getGameServerDetail)
	return r
}

func createGameServerBuild(c *gin.Context) {
	var gsb mpsv1alpha1.GameServerBuild
	err := c.BindJSON(&gsb)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err = kubeClient.Create(ctx, &gsb)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gsb)
}

func listGameServeBuilds(c *gin.Context) {
	var gsbList mpsv1alpha1.GameServerBuildList
	err := kubeClient.List(ctx, &gsbList)
	if err != nil {
		log.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	} else {
		c.JSON(http.StatusOK, gsbList)
	}
}

func getGameServerBuild(c *gin.Context) {
	var gsb mpsv1alpha1.GameServerBuild
	err := kubeClient.Get(ctx, client.ObjectKey{Name: c.Param(buildNameParam), Namespace: namespace}, &gsb)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			log.Error(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
	} else {
		c.JSON(http.StatusOK, gsb)
	}
}

func listGameServersForBuild(c *gin.Context) {
	buildName := c.Param(buildNameParam)
	var gsList mpsv1alpha1.GameServerList
	err := kubeClient.List(ctx, &gsList, client.MatchingLabels{"BuildName": buildName})
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			log.Error(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
	} else {
		c.JSON(http.StatusOK, gsList)
	}
}

func getGameServer(c *gin.Context) {
	gameServerName := c.Param(gameServerNameParam)
	var gs mpsv1alpha1.GameServer
	err := kubeClient.Get(ctx, client.ObjectKey{Name: gameServerName, Namespace: namespace}, &gs)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			log.Error(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
	} else {
		c.JSON(http.StatusOK, gs)
	}
}

func deleteGameServer(c *gin.Context) {
	gameServerName := c.Param(gameServerNameParam)
	var gs mpsv1alpha1.GameServer
	err := kubeClient.Get(ctx, client.ObjectKey{Name: gameServerName, Namespace: namespace}, &gs)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			log.Error(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
	} else {
		err = kubeClient.Delete(ctx, &gs)
		if err != nil {
			log.Error(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusOK, gin.H{"message": "Game server deleted"})
		}
	}
}

func patchGameServerBuild(c *gin.Context) {
	var gsb mpsv1alpha1.GameServerBuild
	err := kubeClient.Get(ctx, client.ObjectKey{Name: c.Param(buildNameParam), Namespace: namespace}, &gsb)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			log.Error(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	var m map[string]interface{}
	err = json.NewDecoder(c.Request.Body).Decode(&m)
	defer c.Request.Body.Close()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	newStandingByInterface := m["standingBy"]
	newMaxInterface := m["max"]

	// values on the map seem to be unmarshaled as float64
	newStandingBy, ok := newStandingByInterface.(float64)
	if !ok {
		log.Info("standingBy not a number")
		c.JSON(http.StatusBadRequest, gin.H{"error": "standingBy not a number"})
		return
	}
	newMax, ok := newMaxInterface.(float64)
	if !ok {
		log.Info("max not a number")
		c.JSON(http.StatusBadRequest, gin.H{"error": "max not a number"})
		return
	}
	if newStandingBy > newMax {
		log.Info("standingBy > max")
		c.JSON(http.StatusBadRequest, gin.H{"error": "standingBy > max"})
		return
	}
	gsb.Spec.Max = int(newMax)
	gsb.Spec.StandingBy = int(newStandingBy)
	err = kubeClient.Update(ctx, &gsb)
	if err != nil {
		log.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	} else {
		c.JSON(http.StatusOK, gsb)
	}
}

func deleteGameServerBuild(c *gin.Context) {
	var gsb mpsv1alpha1.GameServerBuild
	err := kubeClient.Get(ctx, client.ObjectKey{Name: c.Param(buildNameParam), Namespace: namespace}, &gsb)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			log.Error(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	err = kubeClient.Delete(ctx, &gsb)
	if err != nil {
		log.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	} else {
		c.JSON(http.StatusOK, gin.H{"message": "Game server build deleted"})
	}
}

func listGameServerDetailsForBuild(c *gin.Context) {
	buildName := c.Param(buildNameParam)
	var gsdList mpsv1alpha1.GameServerDetailList
	err := kubeClient.List(ctx, &gsdList, client.MatchingLabels{"BuildName": buildName})
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			log.Error(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
	} else {
		c.JSON(http.StatusOK, gsdList)
	}
}

func getGameServerDetail(c *gin.Context) {
	gameServerDetailName := c.Param(gameServerDetailNameParam)
	var gsd mpsv1alpha1.GameServerDetail
	err := kubeClient.Get(ctx, client.ObjectKey{Name: gameServerDetailName, Namespace: namespace}, &gsd)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			log.Error(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
	} else {
		c.JSON(http.StatusOK, gsd)
	}
}
