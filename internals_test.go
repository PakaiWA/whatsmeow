package whatsmeow

import (
	"context"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/PakaiWA/whatsmeow/appstate"
	waBinary "github.com/PakaiWA/whatsmeow/binary"
	"github.com/PakaiWA/whatsmeow/proto/waAdv"
	"github.com/PakaiWA/whatsmeow/store/sqlstore"
	"github.com/PakaiWA/whatsmeow/types"
	"github.com/PakaiWA/whatsmeow/types/events"
	"github.com/PakaiWA/whatsmeow/util/keys"
	waLog "github.com/PakaiWA/whatsmeow/util/log"
)

func runSafe(fn func()) {
	defer func() {
		_ = recover()
	}()
	fn()
}

func TestDangerousInternals_Delegates(t *testing.T) {
	container, err := sqlstore.New(context.Background(), "sqlite3", "file:test_internals?mode=memory&cache=shared&_foreign_keys=on", waLog.Noop)
	if err != nil {
		t.Fatalf("failed to open sqlstore: %v", err)
	}
	t.Cleanup(func() { _ = container.Close() })
	device := container.NewDevice()
	device.ID = &types.JID{User: "1234567890", Server: types.DefaultUserServer}
	device.LID = types.JID{User: "1234567890", Server: types.HiddenUserServer}
	device.Account = &waAdv.ADVSignedDeviceIdentity{
		Details:             []byte("details"),
		AccountSignature:    make([]byte, 64),
		AccountSignatureKey: make([]byte, 32),
		DeviceSignature:     make([]byte, 64),
	}
	if err = device.Save(context.Background()); err != nil {
		t.Fatalf("failed to save device: %v", err)
	}
	cli := NewClient(device, waLog.Noop)
	cli.SynchronousAck = true
	intCli := cli.DangerousInternals()
	if intCli == nil || intCli.c != cli {
		t.Fatalf("expected DangerousInternals() to return wrapper with cli")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	runSafe(func() { intCli.FetchAppState(ctx, appstate.WAPatchRegular, false, false) })
	runSafe(func() { intCli.HandleAppStateRecovery(ctx, types.MessageID("test-id"), nil) })
	runSafe(func() {
		intCli.ApplyAppStatePatches(ctx, appstate.WAPatchRegular, appstate.HashState{}, nil, false, nil)
	})
	runSafe(func() { intCli.CollectEventsToDispatch(ctx, appstate.WAPatchRegular, nil, false, nil) })
	runSafe(func() { intCli.FilterContacts(nil) })
	runSafe(func() { intCli.DispatchAppState(ctx, appstate.WAPatchRegular, appstate.Mutation{}, false) })
	runSafe(func() { intCli.DownloadExternalAppStateBlob(ctx, nil) })
	runSafe(func() { intCli.FetchAppStatePatches(ctx, appstate.WAPatchRegular, 0, false) })
	runSafe(func() {
		intCli.RequestAppStateKeys(ctx, nil)
		intCli.RequestMissingAppStateKeys(ctx, &appstate.PatchList{})
	})
	runSafe(func() { intCli.SendAppState(ctx, appstate.PatchInfo{}, false) })
	runSafe(func() { intCli.HandleDecryptedArmadillo(ctx, nil, nil, 0) })
	runSafe(func() { intCli.GetBroadcastListParticipants(ctx, types.NewJID("1234567890", types.DefaultUserServer)) })
	runSafe(func() { intCli.GetStatusBroadcastRecipients(ctx) })
	runSafe(func() { intCli.HandleCallEvent(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() { intCli.SetTransport(nil, SetProxyOptions{}) })
	runSafe(func() { intCli.GetSocketWaitChan() })
	runSafe(func() { intCli.CloseSocketWaitChan() })
	runSafe(func() { intCli.GetOwnID() })
	runSafe(func() { intCli.GetOwnLID() })
	runSafe(func() { intCli.GetUserAgent() })
	runSafe(func() { intCli.Connect(ctx) })
	runSafe(func() { intCli.UnlockedConnect(ctx) })
	runSafe(func() { intCli.OnDisconnect(ctx, nil, false) })
	runSafe(func() { intCli.ExpectDisconnect() })
	runSafe(func() { intCli.ResetExpectedDisconnect() })
	runSafe(func() { intCli.IsExpectedDisconnect() })
	runSafe(func() { intCli.AutoReconnect(ctx) })
	runSafe(func() { intCli.UnlockedDisconnect() })
	runSafe(func() { intCli.MakeFrameHandler(nil) })
	runSafe(func() { intCli.HandleFrame(ctx, nil, nil) })
	go runSafe(func() { intCli.HandlerQueueLoop(ctx, ctx, nil, nil) })
	runSafe(func() { intCli.SendNodeAndGetData(ctx, waBinary.Node{}) })
	runSafe(func() { intCli.SendNode(ctx, waBinary.Node{}) })
	runSafe(func() { intCli.DispatchEvent(nil) })
	runSafe(func() { intCli.GetUnifiedSessionID() })
	runSafe(func() { intCli.SendUnifiedSession() })
	runSafe(func() { intCli.HandleStreamError(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() { intCli.HandleIB(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() { intCli.HandleConnectFailure(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() { intCli.HandleConnectSuccess(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() { intCli.GenerateCsToken(ctx, types.NewJID("1234567890", types.DefaultUserServer)) })
	runSafe(func() { intCli.StoreNCTSalt(ctx, nil) })
	runSafe(func() { intCli.ClearNCTSalt(ctx) })
	runSafe(func() { intCli.DownloadAndDecrypt(ctx, "", nil, MediaType(""), nil, nil) })
	runSafe(func() { intCli.DownloadPossiblyEncryptedMediaWithRetries(ctx, "", nil) })
	runSafe(func() { intCli.DoMediaDownloadRequest(ctx, "") })
	runSafe(func() { intCli.DownloadMedia(ctx, "") })
	runSafe(func() { intCli.DownloadEncryptedMedia(ctx, "", nil) })
	runSafe(func() { intCli.DownloadAndDecryptToFile(ctx, "", nil, MediaType(""), nil, nil, nil) })
	runSafe(func() { intCli.DownloadPossiblyEncryptedMediaWithRetriesToFile(ctx, "", nil, nil) })
	runSafe(func() { intCli.DownloadMediaToFile(ctx, "", nil) })
	runSafe(func() { intCli.DownloadEncryptedMediaToFile(ctx, "", nil, nil) })
	runSafe(func() {
		intCli.SendGroupIQ(ctx, infoQueryType(""), types.NewJID("1234567890", types.DefaultUserServer), waBinary.Node{})
	})
	runSafe(func() { intCli.CacheGroupInfo(nil, false) })
	runSafe(func() { intCli.GetGroupInfo(ctx, types.NewJID("1234567890", types.DefaultUserServer), false) })
	runSafe(func() { intCli.GetCachedGroupData(ctx, types.NewJID("1234567890", types.DefaultUserServer)) })
	runSafe(func() { intCli.ParseGroupNode(&waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() {
		intCli.ParseGroupCreate(&waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}})
	})
	runSafe(func() { intCli.ParseGroupChange(&waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() { intCli.UpdateGroupParticipantCache(nil) })
	runSafe(func() { intCli.ParseGroupNotification(&waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() { intCli.DoHandshake(nil, keys.KeyPair{}) })
	go runSafe(func() { intCli.KeepAliveLoop(ctx, ctx) })
	runSafe(func() { intCli.SendKeepAlive(ctx) })
	runSafe(func() { intCli.RefreshMediaConn(ctx, false) })
	runSafe(func() { intCli.QueryMediaConn(ctx) })
	runSafe(func() {
		intCli.HandleMediaRetryNotification(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}})
	})
	runSafe(func() { intCli.HandleEncryptedMessage(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() { intCli.HandleUnencryptedMessage(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() { intCli.ParseMessageSource(&waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}, false) })
	runSafe(func() { intCli.ParseMsgBotInfo(waBinary.Node{}) })
	runSafe(func() { intCli.ParseMsgMetaInfo(waBinary.Node{}) })
	runSafe(func() { intCli.ParseMessageInfo(&waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() {
		intCli.HandlePlaintextMessage(ctx, nil, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}})
	})
	runSafe(func() {
		intCli.MigrateSessionStore(ctx, types.NewJID("1234567890", types.DefaultUserServer), types.NewJID("1234567890", types.DefaultUserServer))
	})
	runSafe(func() { intCli.DecryptMessages(ctx, nil, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() { intCli.ClearUntrustedIdentity(ctx, types.NewJID("1234567890", types.DefaultUserServer)) })
	runSafe(func() { intCli.BufferedDecrypt(ctx, nil, time.Now(), nil) })
	runSafe(func() {
		intCli.DecryptDM(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}, types.NewJID("1234567890", types.DefaultUserServer), false, time.Now())
	})
	runSafe(func() {
		intCli.DecryptGroupMsg(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}, types.NewJID("1234567890", types.DefaultUserServer), types.NewJID("1234567890", types.DefaultUserServer), time.Now())
	})
	runSafe(func() {
		intCli.HandleSenderKeyDistributionMessage(ctx, types.NewJID("1234567890", types.DefaultUserServer), types.NewJID("1234567890", types.DefaultUserServer), nil)
	})
	go runSafe(func() { intCli.HandleHistorySyncNotificationLoop() })
	runSafe(func() { intCli.HandleAppStateSyncKeyShare(ctx, nil) })
	runSafe(func() { intCli.HandlePlaceholderResendResponse(nil) })
	runSafe(func() { intCli.HandleProtocolMessage(ctx, nil, nil) })
	runSafe(func() { intCli.ProcessProtocolParts(ctx, nil, nil) })
	runSafe(func() { intCli.StoreMessageSecret(ctx, nil, nil) })
	runSafe(func() { intCli.StoreHistoricalMessageSecrets(ctx, nil) })
	runSafe(func() { intCli.StoreLIDSyncMessage(ctx, nil) })
	runSafe(func() { intCli.StoreGlobalSettings(ctx, nil) })
	runSafe(func() { intCli.StoreCompanionMetaNonce(ctx, "") })
	runSafe(func() { intCli.StoreHistoricalPNLIDMappings(ctx, nil) })
	runSafe(func() { intCli.HandleDecryptedMessage(ctx, nil, nil, 0) })
	runSafe(func() {
		intCli.EncryptWASAMessage(ctx, types.NewJID("1234567890", types.DefaultUserServer), types.MessageID("test-id"), nil)
	})
	runSafe(func() { intCli.DecryptMsgSecret(ctx, nil, MsgSecretType(""), nil, nil) })
	runSafe(func() {
		intCli.EncryptMsgSecret(ctx, types.NewJID("1234567890", types.DefaultUserServer), types.NewJID("1234567890", types.DefaultUserServer), types.NewJID("1234567890", types.DefaultUserServer), types.MessageID("test-id"), MsgSecretType(""), nil)
	})
	runSafe(func() {
		intCli.DecryptBotMessage(ctx, nil, nil, types.MessageID("test-id"), types.NewJID("1234567890", types.DefaultUserServer), nil)
	})
	runSafe(func() { intCli.SendMexIQ(ctx, "", nil) })
	runSafe(func() { intCli.GetNewsletterInfo(ctx, nil, false) })
	runSafe(func() { intCli.HandleEncryptNotification(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() {
		intCli.HandleAppStateNotification(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}})
	})
	runSafe(func() { intCli.HandlePictureNotification(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() { intCli.HandleDeviceNotification(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() {
		intCli.HandleFBDeviceNotification(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}})
	})
	runSafe(func() {
		intCli.HandleOwnDevicesNotification(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}, types.NewJID("1234567890", types.DefaultUserServer))
	})
	runSafe(func() { intCli.HandleBlocklist(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() {
		intCli.HandleAccountSyncNotification(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}})
	})
	runSafe(func() {
		intCli.HandlePrivacyTokenNotification(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}})
	})
	runSafe(func() { intCli.ParseNewsletterMessages(&waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() {
		intCli.HandleNewsletterNotification(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}})
	})
	runSafe(func() { intCli.HandleMexNotification(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() { intCli.HandleStatusNotification(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() { intCli.HandleNotification(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() {
		intCli.TryHandleCodePairNotification(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}})
	})
	runSafe(func() {
		intCli.HandleCodePairNotification(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}})
	})
	runSafe(func() { intCli.HandleIQ(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() { intCli.HandlePairDevice(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() { intCli.RotateADVSecret(ctx) })
	runSafe(func() { intCli.GetQRClientType() })
	runSafe(func() { intCli.MakeQRData(nil, PairClientType("")) })
	runSafe(func() { intCli.HandlePairSuccess(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() {
		intCli.HandlePair(ctx, nil, "", "", "", types.NewJID("1234567890", types.DefaultUserServer), types.NewJID("1234567890", types.DefaultUserServer))
	})
	runSafe(func() { intCli.SendPairError(ctx, "", 0, "") })
	runSafe(func() { intCli.HandlePasskeyNotification(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() {
		intCli.TryHandlePasskeyContinuationNotification(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}})
	})
	runSafe(func() {
		intCli.HandlePasskeyContinuationNotification(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}})
	})
	runSafe(func() { intCli.GetCompanionRef(ctx) })
	runSafe(func() { intCli.GetPasskeyRequestOptions(ctx) })
	runSafe(func() { intCli.GetServerPreKeyCount(ctx) })
	runSafe(func() { intCli.UploadPreKeys(ctx, false) })
	runSafe(func() { intCli.FetchPreKeysNoError(ctx, nil) })
	runSafe(func() { intCli.FetchPreKeys(ctx, nil) })
	runSafe(func() { intCli.HandleChatState(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() { intCli.HandlePresence(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() { intCli.ParsePrivacySettings(&waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}, nil) })
	runSafe(func() {
		intCli.HandlePrivacySettingsNotification(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}})
	})
	runSafe(func() { intCli.HandleReceipt(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() {
		intCli.HandleGroupedReceipt(events.Receipt{}, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}})
	})
	runSafe(func() { intCli.ParseReceipt(&waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() { intCli.BackgroundIfAsyncAck(nil) })
	runSafe(func() { intCli.MaybeDeferredAck(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() { intCli.SendAck(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}, 0) })
	runSafe(func() { intCli.SendMessageReceipt(ctx, nil, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() { intCli.ShouldIncludeReportingToken(nil) })
	runSafe(func() {
		intCli.GetMessageReportingToken(nil, nil, types.NewJID("1234567890", types.DefaultUserServer), types.NewJID("1234567890", types.DefaultUserServer), types.MessageID("test-id"))
	})
	runSafe(func() { intCli.GenerateRequestID() })
	runSafe(func() { intCli.ClearResponseWaiters(&waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() { intCli.WaitResponse("") })
	runSafe(func() { ch := make(chan *waBinary.Node, 1); intCli.CancelResponse("", ch) })
	runSafe(func() { intCli.ReceiveResponse(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() { intCli.SendIQAsyncAndGetData(ctx, nil) })
	runSafe(func() { intCli.SendIQAsync(ctx, infoQuery{}) })
	runSafe(func() { intCli.SendIQ(ctx, infoQuery{}) })
	runSafe(func() {
		intCli.RetryFrame(ctx, "", "", nil, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}, time.Millisecond)
	})
	runSafe(func() {
		intCli.AddRecentMessage(ctx, types.NewJID("1234567890", types.DefaultUserServer), types.MessageID("test-id"), nil, nil)
	})
	runSafe(func() {
		intCli.GetRecentMessage(types.NewJID("1234567890", types.DefaultUserServer), types.MessageID("test-id"))
	})
	runSafe(func() { intCli.GetMessageForRetry(ctx, nil, types.MessageID("test-id")) })
	runSafe(func() { intCli.ShouldRecreateSession(ctx, 0, types.NewJID("1234567890", types.DefaultUserServer)) })
	runSafe(func() {
		intCli.TryHandleRetryReceipt(ctx, nil, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}})
	})
	runSafe(func() { intCli.HandleRetryReceipt(ctx, nil, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() { intCli.CancelDelayedRequestFromPhone(types.MessageID("test-id")) })
	runSafe(func() { intCli.DelayedRequestMessageFromPhone(nil) })
	runSafe(func() { intCli.ImmediateRequestMessageFromPhone(ctx, nil) })
	runSafe(func() { intCli.ClearDelayedMessageRequests() })
	runSafe(func() {
		intCli.SendRetryReceipt(ctx, &waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}, nil, false)
	})
	runSafe(func() {
		intCli.SendNewsletter(ctx, types.NewJID("1234567890", types.DefaultUserServer), types.MessageID("test-id"), nil, "", nil)
	})
	runSafe(func() {
		intCli.SendGroup(ctx, types.NewJID("1234567890", types.DefaultUserServer), types.NewJID("1234567890", types.DefaultUserServer), nil, types.MessageID("test-id"), nil, nil, nodeExtraParams{})
	})
	runSafe(func() {
		intCli.SendPeerMessage(ctx, types.NewJID("1234567890", types.DefaultUserServer), types.MessageID("test-id"), nil, nil)
	})
	runSafe(func() {
		intCli.SendDM(ctx, types.NewJID("1234567890", types.DefaultUserServer), types.NewJID("1234567890", types.DefaultUserServer), types.MessageID("test-id"), nil, nil, nodeExtraParams{})
	})
	runSafe(func() {
		intCli.PreparePeerMessageNode(ctx, types.NewJID("1234567890", types.DefaultUserServer), types.MessageID("test-id"), nil, nil)
	})
	runSafe(func() { intCli.GetMessageContent(waBinary.Node{}, nil, waBinary.Attrs{}, false, nodeExtraParams{}) })
	runSafe(func() {
		intCli.PrepareMessageNode(ctx, types.NewJID("1234567890", types.DefaultUserServer), types.MessageID("test-id"), nil, nil, nil, nil, nil, nodeExtraParams{})
	})
	runSafe(func() { intCli.MakeDeviceIdentityNode() })
	runSafe(func() { intCli.EncryptMessageForDevices(ctx, nil, "", nil, nil, waBinary.Attrs{}) })
	runSafe(func() {
		intCli.EncryptMessageForDeviceAndWrap(ctx, nil, types.NewJID("1234567890", types.DefaultUserServer), types.NewJID("1234567890", types.DefaultUserServer), nil, waBinary.Attrs{}, nil)
	})
	runSafe(func() {
		intCli.EncryptMessageForDevice(ctx, nil, types.NewJID("1234567890", types.DefaultUserServer), nil, waBinary.Attrs{}, nil)
	})
	runSafe(func() {
		intCli.SendGroupV3(ctx, types.NewJID("1234567890", types.DefaultUserServer), types.NewJID("1234567890", types.DefaultUserServer), types.MessageID("test-id"), nil, messageAttrs{}, nil, nil)
	})
	runSafe(func() {
		intCli.SendDMV3(ctx, types.NewJID("1234567890", types.DefaultUserServer), types.NewJID("1234567890", types.DefaultUserServer), types.MessageID("test-id"), nil, messageAttrs{}, nil, nil)
	})
	runSafe(func() {
		intCli.PrepareMessageNodeV3(ctx, types.NewJID("1234567890", types.DefaultUserServer), types.NewJID("1234567890", types.DefaultUserServer), types.MessageID("test-id"), nil, nil, messageAttrs{}, nil, nil, nil)
	})
	runSafe(func() {
		intCli.EncryptMessageForDevicesV3(ctx, nil, types.NewJID("1234567890", types.DefaultUserServer), "", nil, nil, nil, waBinary.Attrs{})
	})
	runSafe(func() {
		intCli.EncryptMessageForDeviceAndWrapV3(ctx, nil, nil, nil, types.NewJID("1234567890", types.DefaultUserServer), nil, waBinary.Attrs{})
	})
	runSafe(func() {
		intCli.EncryptMessageForDeviceV3(ctx, nil, nil, nil, types.NewJID("1234567890", types.DefaultUserServer), nil, waBinary.Attrs{})
	})
	runSafe(func() { intCli.ResolveTCTokenStorageLID(ctx, types.NewJID("1234567890", types.DefaultUserServer)) })
	runSafe(func() { intCli.GetTCTokenSenderTS(types.NewJID("1234567890", types.DefaultUserServer)) })
	runSafe(func() {
		intCli.ValidateAndSetTCTokenSenderTS(types.NewJID("1234567890", types.DefaultUserServer), time.Now())
	})
	runSafe(func() { intCli.SetTCTokenSenderTS(types.NewJID("1234567890", types.DefaultUserServer), time.Now()) })
	runSafe(func() { intCli.UnlockedCleanupTCTokenSenderTSMap() })
	runSafe(func() { intCli.EnsureTCToken(ctx, types.NewJID("1234567890", types.DefaultUserServer)) })
	runSafe(func() { intCli.DeleteExpiredPrivacyTokens() })
	runSafe(func() {
		intCli.IssuePrivacyTokenAndSave(types.NewJID("1234567890", types.DefaultUserServer), time.Now())
	})
	runSafe(func() { intCli.IssuePrivacyToken(ctx, types.NewJID("1234567890", types.DefaultUserServer), time.Now()) })
	runSafe(func() { intCli.RawUpload(ctx, nil, 0, nil, MediaType(""), false, nil) })
	runSafe(func() { intCli.ParseBusinessProfile(&waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
	runSafe(func() { intCli.HandleHistoricalPushNames(ctx, nil) })
	runSafe(func() {
		intCli.UpdatePushName(ctx, types.NewJID("1234567890", types.DefaultUserServer), types.NewJID("1234567890", types.DefaultUserServer), nil, "")
	})
	runSafe(func() {
		intCli.UpdateBusinessName(ctx, types.NewJID("1234567890", types.DefaultUserServer), types.NewJID("1234567890", types.DefaultUserServer), nil, "")
	})
	runSafe(func() { intCli.GetFBIDDevicesInternal(ctx, nil) })
	runSafe(func() { intCli.GetFBIDDevices(ctx, nil) })
	runSafe(func() { intCli.Usync(ctx, nil, "", "", nil) })
	runSafe(func() { intCli.ParseBlocklist(&waBinary.Node{Tag: "receipt", Attrs: waBinary.Attrs{}}) })
}
