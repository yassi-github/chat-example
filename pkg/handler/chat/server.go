package chat

import (
	"context"
	"sync"

	"github.com/bufbuild/connect-go"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/sivchari/chat-example/pkg/domain/entity"
	"github.com/sivchari/chat-example/pkg/log"
	"github.com/sivchari/chat-example/pkg/usecase/chat"
	"github.com/sivchari/chat-example/proto/proto"
	"github.com/sivchari/chat-example/proto/proto/protoconnect"
)

type Stream struct {
	pbStream *connect.ServerStream[proto.JoinRoomResponse]
	close    chan struct{}
}

type server struct {
	logger                  log.Handler
	chatInteractor          chat.Interactor
	streamMapByRoomIDAndKey map[string]map[string]*Stream
	mu                      sync.RWMutex
}

func New(logger log.Handler, chatInteractor chat.Interactor) protoconnect.ChatServiceHandler {
	return &server{
		logger:                  logger,
		chatInteractor:          chatInteractor,
		streamMapByRoomIDAndKey: make(map[string]map[string]*Stream, 0),
	}
}

// request は指定された型(例えば proto.CreateRoomRequest 構造体)である Msg メンバのみが入っている(https://pkg.go.dev/github.com/bufbuild/connect-go#Request)
// その構造体には GetName などのメソッドが定義されている(proto/proto/chat.pb.go)
// s.chatInteractor は chat.Interactor のこと(23行目)
// chat.Interactor とは，importしているが usecase/chat/interactor.go が持つ Interactor インタフェースのこと
// そのインタフェースでは CreateRoom などの関数がプロトタイプ宣言されている
// 実装は同ファイルにあるが，未完成な関数もあるので書いていく必要がある．

// これらを実装すると
// curl localhost:8080/api.ChatService/CreateRoom --header "Content-Type: application/json"  --data '{"name": "roomName"}'
// などのリクエストが通るようになる
// evans で call CreateRoom などとしてリクエストを呼び，データを入力するといった，対話的なリクエストも可能である

func (s *server) CreateRoom(ctx context.Context, req *connect.Request[proto.CreateRoomRequest]) (*connect.Response[proto.CreateRoomResponse], error) {
	room, err := s.chatInteractor.CreateRoom(ctx, req.Msg.GetName())
	if err != nil {
		s.logger.ErrorCtx(ctx, "create room error", "err", err)
		return nil, err
	}
	return connect.NewResponse(&proto.CreateRoomResponse{
		Id: room.ID,
	}), nil
}

func (s *server) GetRoom(ctx context.Context, req *connect.Request[proto.GetRoomRequest]) (*connect.Response[proto.GetRoomResponse], error) {
	room, err := s.chatInteractor.GetRoom(ctx, req.Msg.GetId())
	if err != nil {
		s.logger.ErrorCtx(ctx, "get room error", "err", err)
		return nil, err
	}
	return connect.NewResponse(&proto.GetRoomResponse{
		Room: toProtoRoom(room),
	}), nil
}

func (s *server) ListRoom(ctx context.Context, _ *connect.Request[emptypb.Empty]) (*connect.Response[proto.ListRoomResponse], error) {
	// TODO: s.chatInteracter.ListRoomを実行する
	return connect.NewResponse(&proto.ListRoomResponse{
		// TODO: Rooms: toProtoRooms(rooms)を返却する
	}), nil
}

func (s *server) GetPass(ctx context.Context, _ *connect.Request[emptypb.Empty]) (*connect.Response[proto.GetPassResponse], error) {
	pass, err := s.chatInteractor.GetPass(ctx)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&proto.GetPassResponse{
		Pass: pass,
	}), nil
}

func (s *server) JoinRoom(ctx context.Context, req *connect.Request[proto.JoinRoomRequest], stream *connect.ServerStream[proto.JoinRoomResponse]) error {
	var room *entity.Room
	// TODO: room := s.chatInteracter.GetRoomを実行する
	st := &Stream{}
	// TODO: st := s.getStream(room.ID, req.Msg.GetPass())を実行する
	if st == nil {
		st = &Stream{
			pbStream: stream,
			close:    make(chan struct{}),
		}
		// TODO: s.addStream(room.ID, req.Msg.GetPass(), st)を実行する
		defer func() {
			s.deleteStream(room.ID, req.Msg.GetPass())
			s.logger.InfoCtx(ctx, "delete stream", "stream id", req.Msg.GetPass())
		}()
	}
	select {
	case <-ctx.Done():
		s.logger.InfoCtx(ctx, "leave room", room.ID, ctx.Err())
	case <-st.close:
		s.logger.InfoCtx(ctx, "leave room", room.ID, "close stream")
	}
	return nil
}

func (s *server) LeaveRoom(ctx context.Context, req *connect.Request[proto.LeaveRoomRequest]) (*connect.Response[emptypb.Empty], error) {
	st := &Stream{}
	// TODO: st := s.getStream(req.Msg.GetRoomId(), req.Msg.GetPass())を実行する
	st.close <- struct{}{}
	close(st.close)
	return &connect.Response[emptypb.Empty]{}, nil
}

func (s *server) ListMessage(ctx context.Context, req *connect.Request[proto.ListMessageRequest]) (*connect.Response[proto.ListMessageResponse], error) {
	// TODO: s.chatInteracter.ListMessageを実行する
	return &connect.Response[proto.ListMessageResponse]{Msg: &proto.ListMessageResponse{
		// TODO: Messages: toProtoMessages(messages)を返却する
	}}, nil
}

func (s *server) Chat(ctx context.Context, req *connect.Request[proto.ChatRequest]) (*connect.Response[proto.ChatResponse], error) {
	var room *entity.Room
	// TODO: room := s.chatInteracter.GetRoomを実行する

	if err := s.chatInteractor.SendMessage(ctx, req.Msg.GetMessage().GetRoomId(), req.Msg.GetMessage().GetText()); err != nil {
		s.logger.ErrorCtx(ctx, "send message error", "err", err)
		return nil, err
	}

	eg, ctx := errgroup.WithContext(ctx)
	streams := s.getStreams(room.ID)
	for _, st := range streams {
		// コピーしないとすべて最後のstのみを参照してしまう
		// ref: https://qiita.com/sudix/items/67d4cad08fe88dcb9a6d
		st := st
		_ = st
		eg.Go(func() error {
			// TODO: st.pbStream.Sendを実行する
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		s.logger.ErrorCtx(ctx, "send message error via stream", "err", err)
		return nil, err
	}

	return connect.NewResponse(&proto.ChatResponse{
		// TODO: Message: req.Msg.GetMessage()を返却する
	}), nil
}

func (s *server) addStream(roomID string, key string, stream *Stream) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.streamMapByRoomIDAndKey[roomID]; !ok {
		s.streamMapByRoomIDAndKey[roomID] = make(map[string]*Stream, 0)
	}
	s.streamMapByRoomIDAndKey[roomID][key] = stream
}

func (s *server) deleteStream(roomID, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.streamMapByRoomIDAndKey[roomID], key)
}

func (s *server) getStreams(roomID string) map[string]*Stream {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.streamMapByRoomIDAndKey[roomID]
}

func (s *server) getStream(roomID string, key string) *Stream {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.streamMapByRoomIDAndKey[roomID][key]
}

func toProtoRoom(room *entity.Room) *proto.Room {
	if room == nil {
		return nil
	}
	return &proto.Room{
		Id:   room.ID,
		Name: room.Name,
	}
}

func toProtoRooms(rooms []*entity.Room) []*proto.Room {
	ret := make([]*proto.Room, 0, len(rooms))
	for _, room := range rooms {
		ret = append(ret, toProtoRoom(room))
	}
	return ret
}

func toProtoMessage(message *entity.Message) *proto.Message {
	if message == nil {
		return nil
	}
	return &proto.Message{
		RoomId: message.RoomID,
		Text:   message.Text,
	}
}

func toProtoMessages(messages []*entity.Message) []*proto.Message {
	ret := make([]*proto.Message, 0, len(messages))
	for _, message := range messages {
		ret = append(ret, toProtoMessage(message))
	}
	return ret
}
