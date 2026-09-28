package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/app/usecase"
	"gorm.io/gorm"
)

type Dependencies struct {
	DB             *gorm.DB
	Logger         *slog.Logger
	Metrics        RequestMetrics
	AdminToken     string
	WebhookSecret  string
	GetRaffle      *usecase.GetRaffle
	OpenOrder      *usecase.OpenOrder
	ConfirmPayment *usecase.ConfirmPayment
	ListPurchases  *usecase.ListPurchasesByEmail
	CreateRaffle   *usecase.CreateRaffle
	UpdateRaffle   *usecase.UpdateRaffleStatus
}

func NewRouter(deps Dependencies) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(cors(), gin.Recovery(), observability(deps.Logger, deps.Metrics))
	engine.GET("/health", health)
	engine.GET("/v1/docs", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/v1/docs/index.html")
	})
	engine.GET("/v1/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	v1 := engine.Group("/v1")
	v1.Use(transactionMiddleware(deps.DB, deps.Logger, deps.Metrics))
	h := &handler{
		webhookSecret:  deps.WebhookSecret,
		getRaffle:      deps.GetRaffle,
		openOrder:      deps.OpenOrder,
		confirmPayment: deps.ConfirmPayment,
		listPurchases:  deps.ListPurchases,
		createRaffle:   deps.CreateRaffle,
		updateRaffle:   deps.UpdateRaffle,
		log: func(c *gin.Context, err error) {
			status, _, _ := mapError(err)
			if status >= http.StatusInternalServerError {
				logInternal(deps.Logger, c, err)
			}
		},
	}
	v1.GET("/raffles/:slug", h.GetRaffle)
	v1.POST("/raffles/:slug/orders", h.CreateOrder)
	v1.POST("/payments/webhooks", h.ConfirmPayment)
	v1.GET("/purchases", h.ListPurchases)

	admin := v1.Group("/admin", adminAuth(deps.AdminToken))
	admin.POST("/raffles", h.CreateRaffle)
	admin.PATCH("/raffles/:id", h.UpdateRaffleStatus)
	return engine
}

func health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
