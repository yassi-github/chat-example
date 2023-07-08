package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/rs/cors"

	"github.com/sivchari/chat-example/pkg/handler/chat"
	"github.com/sivchari/chat-example/pkg/handler/healthz"
	messagerepository "github.com/sivchari/chat-example/pkg/infra/repository/message"
	roomrepository "github.com/sivchari/chat-example/pkg/infra/repository/room"
	"github.com/sivchari/chat-example/pkg/log"
	"github.com/sivchari/chat-example/pkg/ulid"
	chatinteractor "github.com/sivchari/chat-example/pkg/usecase/chat"
	"github.com/sivchari/chat-example/proto/proto/protoconnect"
)

func main() {
	os.Exit(run())
}

func run() int {
	const (
		ok = 0
		ng = 1
	)

	// DI
	logger := log.NewHandler(log.LevelInfo, log.WithJSONFormat())
	ulidGenerator := ulid.NewUILDGenerator()
	roomRepository := roomrepository.New()
	messageRepository := messagerepository.New()
	chatInteractor := chatinteractor.New(ulidGenerator, roomRepository, messageRepository)
	healthzServer := healthz.New(logger)
	chatServer := chat.New(logger, chatInteractor)

	mux := http.NewServeMux()
	// ハンドラを追加
	// .proto ファイルから protoc が自動生成したやつ
	// healthz.proto では rpc Check と定義しているので， protoconnect/healthz.connect.go が作られ，
	// そこの HealthzHandler インタフェースで Check 関数が宣言される
	// それを handler/healthz で実装する
	// そのときのレシーバには server 構造体を指定しており，メンバには logやinteractor といった実装に使用する関数のパッケージを指定する
	// よってハンドラの実装では，レシーバの構造体から関数を呼び出していくことで構成するようである
	// また，実装ファイルの New 関数は HealthzHandler インタフェースを返す
	// そして，ここ(main)の冒頭で呼ばれ，ハンドラの引数に与えられる(ハンドラはその引数のインタフェースからメソッドを呼び出す)
	mux.Handle(protoconnect.NewHealthzHandler(healthzServer))
	mux.Handle(protoconnect.NewChatServiceHandler(chatServer))
	// http2に対応する(http2.Server は http.Server と似ているが設定値は違う．アドレスやハンドラは結局 http.Server のほうでやる)
	handler := cors.AllowAll().Handler(h2c.NewHandler(mux, &http2.Server{}))
	srv := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	// コンテキストでkillシグナルを受け取る
	// background() はコンテキストを生成・初期化している
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	// goroutine でhttpサーバを起動
	go func() {
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			logger.ErrorCtx(ctx, "failed to ListenAndServe", "err", err)
		}
	}()

	// cancel によってコンテキストがキャンセルされる待ち？判定？
	<-ctx.Done()

	// httpサーバを安全に閉じるが，タイムアウトを設定している
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(timeoutCtx); err != nil {
		return ng
	}
	return ok
}
