import { derived, writable } from 'svelte/store'

export const LANGUAGE_STORAGE_KEY = 'tether.language'
export const SUPPORTED_LANGUAGES = ['ru', 'en'] as const

export type Language = (typeof SUPPORTED_LANGUAGES)[number]
export type TranslationParams = Record<string, string | number | boolean | null | undefined>
export type TranslationTable = Record<string, string>
export type TranslationDictionaries = Record<Language, Partial<TranslationTable>>

const EN_TRANSLATIONS = {
  'app.bootstrapFailed': 'Bootstrap failed',
  'app.loading': 'Loading the Wails runtime contract and app shell...',
  'app.title': 'tether',

  'common.archPending': '...',
  'common.bindingsCount': '{count} bindings',
  'common.booting': 'booting',
  'common.cancel': 'Cancel',
  'common.callsCount': '{count} calls',
  'common.checking': 'Checking...',
  'common.contractDrift': 'contract drift',
  'common.contractVerified': 'contract verified',
  'common.create': 'Create',
  'common.diagnostics': 'diagnostics',
  'common.hide': 'Hide panel',
  'common.language': 'Language',
  'common.languageEn': 'Английский',
  'common.languageRu': 'Russian',
  'common.methodsCount': '{count} methods',
  'common.notAvailable': 'n/a',
  'common.notOpen': 'not open',
  'common.open': 'Open',
  'common.productLine': 'Desktop-first gRPC debugging workspace',
  'common.productLineFallback': 'Desktop-first gRPC debugging workspace',
  'common.requestsCount': '{count} requests',
  'common.refreshing': 'refreshing...',
  'common.save': 'Save',
  'common.saving': 'Saving...',
  'common.servicesCount': '{count} services',
  'common.validate': 'Validate',
  'common.view': 'view',
  'common.working': 'Working...',

  'diagnostics.contractGuard': 'Contract guard',
  'diagnostics.detail.cause': 'Cause',
  'diagnostics.detail.import': 'Import',
  'diagnostics.detail.source': 'Source',
  'diagnostics.emptyCaptured': 'No diagnostics captured yet.',
  'diagnostics.emptyRecent': 'No diagnostics yet.',
  'diagnostics.manifestsMatch': 'Frontend and backend manifests match.',
  'diagnostics.nextStep': 'Next step: {step}',
  'diagnostics.overlayCopy':
    'Diagnostics stay wired in the app shell; the detailed event feed opens when the diagnostics overlay is active.',
  'diagnostics.overlayTitle': 'Overlay behavior',
  'diagnostics.probeInProgress': 'Wails event bridge probe in progress.',
  'diagnostics.probeNotRun': 'Probe has not run yet.',
  'diagnostics.probeReady': 'Runtime ready',
  'diagnostics.probeStatus': 'Probe status',
  'diagnostics.structuredStatus': 'Structured status',
  'diagnostics.title': 'Diagnostics',
  'diagnostics.code.application_runtime_ready.label': 'Runtime ready',
  'diagnostics.code.application_runtime_ready.message': 'Frontend/backend event bridge verified successfully.',
  'diagnostics.code.application_runtime_ready.nextStep': 'Proceed with Epic 1 runtime features on top of the validated shell.',
  'diagnostics.code.proto_catalog_loaded.message': 'Proto catalog loaded successfully.',
  'diagnostics.code.reflection_catalog_loaded.message': 'Reflection catalog loaded successfully.',
  'diagnostics.code.validation_client_stream_messages_required.message':
    'Client-streaming calls require at least one request message.',
  'diagnostics.code.validation_client_stream_static_sequence_required.message':
    'Client-streaming calls require a request mode before starting.',
  'diagnostics.code.validation_bidi_stream_interactive_required.message':
    'Bidirectional streaming calls require interactive mode before starting.',
  'diagnostics.code.application_stream_send_unavailable.message':
    'The stream is not open for sending anymore.',
  'diagnostics.code.application_stream_half_close_unavailable.message':
    'The local send side is not open for half-close.',
  'diagnostics.code.application_stream_session_not_found.message':
    'The stream has already finished or is no longer active.',

  'endpoint.authority': 'Authority',
  'endpoint.authorityPlaceholder': 'optional override',
  'endpoint.caSecretRef': 'CA secret-ref',
  'endpoint.catalogSource': 'Catalog source',
  'endpoint.clientCertSecretRef': 'Client cert secret-ref',
  'endpoint.clientKeySecretRef': 'Client key secret-ref',
  'endpoint.connectTimeout': 'Connect timeout (ms)',
  'endpoint.checkOutcome.failed': 'failed',
  'endpoint.checkOutcome.not_proven': 'not proven',
  'endpoint.checkOutcome.passed': 'passed',
  'endpoint.checkOutcome.skipped': 'skipped',
  'endpoint.checkStage.grpc_readiness': 'gRPC readiness',
  'endpoint.checkStage.target_resolution': 'target resolution',
  'endpoint.checkStage.tcp_connect': 'TCP connect',
  'endpoint.checkStage.tls_handshake': 'TLS handshake',
  'endpoint.checkMessage.grpc_readiness.failed': 'gRPC readiness check failed.',
  'endpoint.checkMessage.grpc_readiness.not_proven': 'gRPC readiness was not proven.',
  'endpoint.checkMessage.grpc_readiness.passed': 'gRPC readiness was proven.',
  'endpoint.checkMessage.target_resolution.failed': 'Target resolution failed.',
  'endpoint.checkMessage.target_resolution.not_proven': 'Target resolution was not proven.',
  'endpoint.checkMessage.target_resolution.passed': 'Target resolution succeeded.',
  'endpoint.checkMessage.tcp_connect.failed': 'TCP connection failed.',
  'endpoint.checkMessage.tcp_connect.not_proven': 'TCP connection was not proven.',
  'endpoint.checkMessage.tcp_connect.passed': 'TCP connection succeeded.',
  'endpoint.checkMessage.tls_handshake.failed': 'TLS handshake failed.',
  'endpoint.checkMessage.tls_handshake.not_proven': 'TLS handshake was not proven.',
  'endpoint.checkMessage.tls_handshake.passed': 'TLS handshake succeeded.',
  'endpoint.checkMessage.tls_handshake.skipped': 'TLS handshake was skipped.',
  'endpoint.grpcBlocked': 'not proven',
  'endpoint.grpcReady': 'ready',
  'endpoint.importPaths': 'Import paths (optional, one per line)',
  'endpoint.importPathsPlaceholder': '/absolute/path/to/import-root',
  'endpoint.loadProto': 'Load proto catalog',
  'endpoint.loadProtoPending': 'Loading proto catalog...',
  'endpoint.loadReflection': 'Load reflection catalog',
  'endpoint.loadReflectionPending': 'Loading reflection...',
  'endpoint.preflightEmpty': 'Run endpoint preflight to reuse transport, TLS and gRPC readiness checks.',
  'endpoint.preflightTitle': 'Transport preflight',
  'endpoint.protoDirectories': 'Proto directories (one per line)',
  'endpoint.protoDirectoriesPlaceholder': '/absolute/path/to/proto',
  'endpoint.protoFiles': 'Proto files (optional, one per line)',
  'endpoint.protoFilesPlaceholder': '/absolute/path/to/service.proto',
  'endpoint.protoNote': 'Proto catalogs reload only when you click the button below. File watching is out of scope for this slice.',
  'endpoint.requestTimeout': 'Request timeout (ms)',
  'endpoint.runPreflight': 'Run endpoint preflight',
  'endpoint.serverNameOverride': 'Server name override',
  'endpoint.serverNamePlaceholder': 'optional SAN override',
  'endpoint.streamIdleTimeout': 'Stream idle timeout (ms)',
  'endpoint.target': 'Target',
  'endpoint.testPending': 'Testing endpoint...',
  'endpoint.title': 'Эндпоинт',
  'endpoint.transport': 'transport',
  'endpoint.tlsFailed': 'failed',
  'endpoint.tlsMode': 'TLS mode',
  'endpoint.tlsOff': 'off',
  'endpoint.tlsOk': 'ok',
  'endpoint.transportBlocked': 'blocked',
  'endpoint.transportReachable': 'reachable',

  'errors.addProtoSource': 'Add at least one proto directory or file before loading the proto catalog.',
  'errors.bootstrapUnexpected': 'Failed to bootstrap the application shell.',
  'errors.diagnosticsProbeUnexpected': 'Unexpected diagnostics probe failure.',
  'errors.endpointPreflight': 'Endpoint preflight failed unexpectedly.',
  'errors.historyDetail': 'History detail could not be loaded.',
  'errors.metadataObject': 'Metadata overrides must be a JSON object with string values.',
  'errors.metadataStringValue': 'Metadata value for "{key}" must be a string.',
  'errors.metadataValidJson': 'Metadata overrides must be valid JSON.',
  'errors.protoCatalog': 'Proto catalog load failed unexpectedly.',
  'errors.reflectionCatalog': 'Reflection catalog load failed unexpectedly.',
  'errors.requestBodyJson': 'Request body must be valid JSON.',
  'errors.requestSave': 'Request could not be saved.',
  'errors.requestSaveLoadedMethod': 'Select a loaded method before saving a reusable request.',
  'errors.requestSaveUnaryOnly': 'Saved request persistence is wired to the unary request composer in this slice.',
  'errors.clientStreamMessagesArray': 'Client-streaming static sequence must be a JSON array of request messages.',
  'errors.clientStreamMessagesRequired': 'Add at least one request message to the client-streaming static sequence.',
  'errors.clientStreamHalfClose': 'Client stream send side could not be half-closed.',
  'errors.clientStreamHalfCloseUnavailable': 'Start an open interactive client stream before half-closing the send side.',
  'errors.clientStreamSend': 'Client stream message could not be sent.',
  'errors.clientStreamSendUnavailable': 'Start an open interactive client stream before sending a message.',
  'errors.clientStreamStart': 'Client stream could not be started.',
  'errors.bidiStreamHalfClose': 'Bidi stream send side could not be half-closed.',
  'errors.bidiStreamHalfCloseUnavailable': 'Start an open bidi stream before half-closing the send side.',
  'errors.bidiStreamSend': 'Bidi stream message could not be sent.',
  'errors.bidiStreamSendUnavailable': 'Start an open bidi stream before sending a message.',
  'errors.bidiStreamStart': 'Bidi stream could not be started.',
  'errors.selectMethodInvoke': 'Select a method from the loaded catalog before invoking.',
  'errors.selectMethodStream': 'Select a method from the loaded catalog before starting a stream.',
  'errors.streamCancel': 'Stream could not be cancelled.',
  'errors.streamStart': 'Server stream could not be started.',
  'errors.streamUnaryOnly': 'Select a streaming method that matches the requested stream flow.',
  'errors.unaryInvoke': 'Unary call failed unexpectedly.',
  'errors.unaryOnly': 'This invoke surface is still unary-only. Pick a unary method from the catalog.',
  'errors.workspaceCreate': 'Workspace could not be created.',
  'errors.workspaceOpen': 'Workspace could not be opened.',
  'errors.workspaceSave': 'Workspace could not be saved.',
  'errors.workspaceValidate': 'Workspace could not be validated.',

  'footer.notRun': 'not-run',
  'footer.probe': 'probe',
  'footer.stream': 'stream',

  'history.artifacts': 'Artifacts',
  'history.detailTitle': 'Persisted history detail',
  'history.empty': 'Completed calls will appear here with persisted summaries and session log artifacts.',
  'history.loading': 'Loading persisted session artifact...',
  'history.recentTitle': 'Recent call history',
  'history.storedSummary': 'Stored summary',
  'history.structuredLogEvents': 'Structured log events',

  'home.appShellCopy':
    'Four layout regions are active and the frontend is bootstrapped from Go runtime metadata, not browser-only assumptions.',
  'home.appShellTitle': 'App shell',
  'home.contractDrift': 'Contract drift detected and blocked by startup verification.',
  'home.contractGuardTitle': 'Contract guard',
  'home.contractMatch': 'Frontend and backend identifiers match the v1 contract manifest.',
  'home.copy': 'Wails runtime, Svelte shell and diagnostics event bus are wired before the next feature slices.',
  'home.eyebrow': 'UX stabilization',
  'home.openSession': 'Open session shell',
  'home.openWorkspace': 'Open workspace shell',
  'home.primaryFlow': 'Primary flow',
  'home.slices': 'Epic 0 slices',
  'home.slice.0_1.summary': 'Wails shell, Svelte app frame, runtime binding and diagnostics event round-trip.',
  'home.slice.0_2.summary': 'Contract manifest, invoke DTOs and module boundaries encoded in shared runtime metadata.',
  'home.slice.0_3.summary': 'Canonical stream states, overlays and error taxonomy wired into shared code paths.',
  'home.sliceStatus.implemented': 'implemented',
  'home.title': 'Production shell is live',

  'method.catalogEmptyProto': 'Load local proto sources and import paths to build the method tree without server reflection.',
  'method.catalogEmptyReflection': 'Load reflection to build the service tree, request templates and RPC metadata that drive the call flow.',
  'method.catalogTitle': 'Method catalog',
  'method.method': 'Method',
  'method.requestType': 'Request type',
  'method.responseType': 'Response type',
  'method.rpc.bidi_stream': 'Bidi stream',
  'method.rpc.client_stream': 'Client stream',
  'method.rpc.server_stream': 'Server stream',
  'method.rpc.unary': 'Unary',
  'method.types.request': 'request:',
  'method.types.response': 'response:',

  'nav.diagnosticsOverlay': 'Diagnostics',
  'nav.diagnosticsOverlayCopy': 'Runtime events and classified failures.',
  'nav.historyOverlay': 'History',
  'nav.historyOverlayCopy': 'Persisted call summaries.',
  'nav.homeCopy': 'Readiness and project status.',
  'nav.homeTitle': 'Home',
  'nav.overlays': 'Overlays',
  'nav.primaryFlow': 'Primary flow',
  'nav.sessionCopy': 'Stream state, probe and diagnostics.',
  'nav.sessionTitle': 'Session',
  'nav.settingsOverlay': 'Settings',
  'nav.settingsOverlayCopy': 'Language and local preferences.',
  'nav.workspaceCopy': 'Endpoint, catalog, request and response.',
  'nav.workspaceTitle': 'Workspace',

  'request.bodyJson': 'Request body JSON',
  'request.bidiMessageJson': 'Bidi message JSON',
  'request.clientMessageJson': 'Client message JSON',
  'request.clientSequenceJson': 'Static message sequence JSON',
  'request.clientStreamMode': 'Client stream mode',
  'request.clientStreamModeInteractive': 'Interactive',
  'request.clientStreamModeStatic': 'Static sequence',
  'request.composerTitle': 'Request composer',
  'request.empty': 'Pick a method from the loaded catalog to materialize the starter JSON payload.',
  'request.invoke': 'Invoke unary call',
  'request.invokePending': 'Invoking unary call...',
  'request.metadataJson': 'Metadata overrides JSON',
  'request.resetTemplate': 'Reset to template',
  'request.saveRequest': 'Save request',
  'request.saveRequestPending': 'Saving request...',
  'request.savedRequestId': 'Saved request id',
  'request.shortcut': 'Use {shortcut} inside the editors to run the selected unary or streaming method.',
  'request.startStream': 'Start server stream',
  'request.startStreamPending': 'Starting stream...',
  'request.startClientStream': 'Run client stream',
  'request.startClientStreamPending': 'Running client stream...',
  'request.startInteractiveClientStream': 'Start interactive stream',
  'request.startBidiStream': 'Start bidi stream',
  'request.startBidiStreamPending': 'Starting bidi stream...',
  'request.sendClientMessage': 'Send message',
  'request.sendClientMessagePending': 'Sending...',
  'request.sendBidiMessage': 'Send message',
  'request.sendBidiMessagePending': 'Sending...',
  'request.halfClose': 'Half-close send',
  'request.halfClosePending': 'Half-closing...',
  'request.cancelStream': 'Cancel stream',
  'request.cancelStreamPending': 'Cancelling...',
  'request.unsupportedMethod': '{method} is {rpcType}.',
  'request.unsupported': 'Slice 3.4 supports unary, server-streaming, client-streaming and bidi-streaming methods.',

  'response.body': 'Response body',
  'response.empty': 'Headers, status, trailers, unary body and streaming timeline events appear here after invocation starts.',
  'response.finished': 'Finished',
  'response.headers': 'Headers',
  'response.panelTitle': 'Response panel',
  'response.started': 'Started',
  'response.status': 'Status',
  'response.trailers': 'Trailers',

  'session.backToWorkspace': 'Back to workspace shell',
  'session.copy': 'UI shell and Go runtime share stream states, terminal outcomes and diagnostics taxonomy.',
  'session.diagnosticsProbe': 'Diagnostics probe',
  'session.errorTaxonomy': 'Error taxonomy',
  'session.eyebrow': 'Slice 0.3',
  'session.lastAcknowledgement': 'Last event bridge acknowledgement:',
  'session.noConditions': 'no conditions',
  'session.probeNotCompleted': 'Diagnostics probe has not completed yet.',
  'session.recentDiagnostics': 'Recent diagnostics',
  'session.title': 'Canonical session state model',
  'session.waitingEmission': 'Waiting for runtime event emission.',

  'source.proto': 'proto sources',
  'source.reflection': 'server reflection',

  'stream.cancelRequested': 'Cancel requested for {callId}.',
  'stream.condition.truncated': 'truncated',
  'stream.completedClosed': '{rpcType} saved to history as {callId}.',
  'stream.completedOther': '{rpcType} finished as {state} with {status}.',
  'stream.contextLocked': 'Cancel the active stream {callId} before changing endpoint, catalog or method context.',
  'stream.contextLockedInfo': 'Active stream {callId} is pinned here until it closes or is cancelled.',
  'stream.emptyFilteredTimeline': 'No stream events match the current timeline filters.',
  'stream.emptyTimeline': 'Live stream events will append here as headers, messages and trailers arrive.',
  'stream.halfClosedLocalReceiving': 'Local send side is closed; incoming responses can still arrive.',
  'stream.halfCloseRequested': 'Client send side half-closed for {callId}.',
  'stream.messageSent': 'Sent client message #{index} for {callId}.',
  'stream.started': '{rpcType} started as {callId}.',
  'stream.truncatedWarning': 'This session is marked truncated; the visible live feed may be incomplete.',
  'streamState.cancelled': 'cancelled',
  'streamState.closed': 'closed',
  'streamState.connecting': 'connecting',
  'streamState.error': 'error',
  'streamState.half_closed_local': 'half-closed local',
  'streamState.half_closed_remote': 'half-closed remote',
  'streamState.idle': 'idle',
  'streamState.open': 'open',

  'tls.custom_ca': 'Пользовательский CA',
  'tls.mtls': 'mTLS',
  'tls.plaintext': 'Plaintext',
  'tls.system_ca': 'System CA',

  'timeline.allDirections': 'all',
  'timeline.allKinds': 'all',
  'timeline.direction': 'Direction',
  'timeline.eventsCount': '{count} events',
  'timeline.jumpToError': 'Jump to error',
  'timeline.jumpToLive': 'Jump to live',
  'timeline.kind': 'Kind',
  'timeline.received': 'received',
  'timeline.sent': 'sent',
  'timeline.windowCount': '{count}/{total} rendered',

  'topbar.contract': 'contract',
  'topbar.eyebrow': 'UX / i18n stabilization',
  'topbar.probe': 'Probe event bridge',
  'topbar.probePending': 'Probing event bridge...',
  'topbar.runtime': 'runtime',

  'workspace.created': 'Created workspace {name} at {manifestPath}.{backup}',
  'workspace.backupSuffix': ' Backup: {path}.',
  'workspace.fileTitle': 'Workspace file',
  'workspace.id': 'Workspace id',
  'workspace.infoDiagnostics': 'Structured diagnostics still flow into the diagnostics panel when the runtime can classify the failure.',
  'workspace.manifest': 'Manifest',
  'workspace.name': 'Name',
  'workspace.namePlaceholder': 'workspace name',
  'workspace.opened': 'Opened workspace {name} at {manifestPath}.{backup}',
  'workspace.path': 'Workspace path',
  'workspace.pathPlaceholder': '/absolute/path/to/workspace',
  'workspace.saved': 'Saved workspace {name} at {manifestPath}.{backup}',
  'workspace.validationIssues': 'Workspace draft has validation issues: {count}.',
  'workspace.validationPassed': 'Workspace draft passed open/save validation.',

  'workspaceView.copy':
    'Reflection, proto catalogs, unary calls and streaming flows share one tighter workspace surface.',
  'workspaceView.eyebrow': 'Slice 4.1',
  'workspaceView.title': 'gRPC workspace',

  'call.unarySaved': 'Unary call saved to history as {callId}.',
  'call.unarySavedWithStatus': 'Unary call completed with {status} and was saved to history as {callId}.',
  'call.requestSaved': 'Saved request {id} to {path}.',

  'view.home': 'Home',
  'view.session': 'Session',
  'view.workspace': 'Workspace',
} as const

type TranslationKey = keyof typeof EN_TRANSLATIONS

const RU_TRANSLATIONS: Record<TranslationKey, string> = {
  'app.bootstrapFailed': 'Ошибка запуска',
  'app.loading': 'Загружаем runtime-контракт Wails и оболочку приложения...',
  'app.title': 'tether',

  'common.archPending': '...',
  'common.bindingsCount': '{count} привязок',
  'common.booting': 'загрузка',
  'common.cancel': 'Отмена',
  'common.callsCount': '{count} вызовов',
  'common.checking': 'Проверяем...',
  'common.contractDrift': 'контракт изменился',
  'common.contractVerified': 'контракт проверен',
  'common.create': 'Создать',
  'common.diagnostics': 'диагностика',
  'common.hide': 'Скрыть панель',
  'common.language': 'Язык',
  'common.languageEn': 'English',
  'common.languageRu': 'Русский',
  'common.methodsCount': '{count} методов',
  'common.notAvailable': 'н/д',
  'common.notOpen': 'не открыт',
  'common.open': 'Открыть',
  'common.productLine': 'Локальное рабочее место для отладки gRPC',
  'common.productLineFallback': 'Локальное рабочее место для отладки gRPC',
  'common.requestsCount': '{count} запросов',
  'common.refreshing': 'обновляем...',
  'common.save': 'Сохранить',
  'common.saving': 'Сохраняем...',
  'common.servicesCount': '{count} сервисов',
  'common.validate': 'Проверить',
  'common.view': 'экран',
  'common.working': 'Выполняем...',

  'diagnostics.contractGuard': 'Проверка контракта',
  'diagnostics.detail.cause': 'Причина',
  'diagnostics.detail.import': 'Импорт',
  'diagnostics.detail.source': 'Источник',
  'diagnostics.emptyCaptured': 'Диагностика пока не получена.',
  'diagnostics.emptyRecent': 'Диагностики пока нет.',
  'diagnostics.manifestsMatch': 'Манифесты frontend и backend совпадают.',
  'diagnostics.nextStep': 'Следующий шаг: {step}',
  'diagnostics.overlayCopy':
    'Диагностика остается подключенной к оболочке; подробная лента событий открывается в панели диагностики.',
  'diagnostics.overlayTitle': 'Поведение панели',
  'diagnostics.probeInProgress': 'Проверяем мост событий Wails.',
  'diagnostics.probeNotRun': 'Проверка еще не запускалась.',
  'diagnostics.probeReady': 'Среда готова',
  'diagnostics.probeStatus': 'Статус проверки',
  'diagnostics.structuredStatus': 'Структурный статус',
  'diagnostics.title': 'Диагностика',
  'diagnostics.code.application_runtime_ready.label': 'Среда готова',
  'diagnostics.code.application_runtime_ready.message': 'Мост событий между frontend и backend успешно проверен.',
  'diagnostics.code.application_runtime_ready.nextStep': 'Можно продолжать runtime-функции поверх проверенной оболочки.',
  'diagnostics.code.proto_catalog_loaded.message': 'Proto-каталог успешно загружен.',
  'diagnostics.code.reflection_catalog_loaded.message': 'Каталог server reflection успешно загружен.',
  'diagnostics.code.validation_client_stream_messages_required.message':
    'Для клиентского потока нужно минимум одно сообщение запроса.',
  'diagnostics.code.validation_client_stream_static_sequence_required.message':
    'Для клиентского потока нужно выбрать режим запроса перед запуском.',
  'diagnostics.code.validation_bidi_stream_interactive_required.message':
    'Двунаправленный поток нужно запускать в интерактивном режиме.',
  'diagnostics.code.application_stream_send_unavailable.message':
    'Поток больше не открыт для отправки.',
  'diagnostics.code.application_stream_half_close_unavailable.message':
    'Локальная отправка сейчас не открыта для закрытия.',
  'diagnostics.code.application_stream_session_not_found.message':
    'Поток уже завершен или больше не активен.',

  'endpoint.authority': 'Authority',
  'endpoint.authorityPlaceholder': 'опциональное переопределение',
  'endpoint.caSecretRef': 'Ссылка на CA-секрет',
  'endpoint.catalogSource': 'Источник каталога',
  'endpoint.clientCertSecretRef': 'Ссылка на клиентский сертификат',
  'endpoint.clientKeySecretRef': 'Ссылка на клиентский ключ',
  'endpoint.connectTimeout': 'Таймаут подключения (мс)',
  'endpoint.checkOutcome.failed': 'ошибка',
  'endpoint.checkOutcome.not_proven': 'не подтверждено',
  'endpoint.checkOutcome.passed': 'пройдено',
  'endpoint.checkOutcome.skipped': 'пропущено',
  'endpoint.checkStage.grpc_readiness': 'готовность gRPC',
  'endpoint.checkStage.target_resolution': 'разрешение адреса',
  'endpoint.checkStage.tcp_connect': 'TCP-подключение',
  'endpoint.checkStage.tls_handshake': 'TLS-рукопожатие',
  'endpoint.checkMessage.grpc_readiness.failed': 'Проверка готовности gRPC завершилась ошибкой.',
  'endpoint.checkMessage.grpc_readiness.not_proven': 'Готовность gRPC не подтверждена.',
  'endpoint.checkMessage.grpc_readiness.passed': 'Готовность gRPC подтверждена.',
  'endpoint.checkMessage.target_resolution.failed': 'Разрешение адреса завершилось ошибкой.',
  'endpoint.checkMessage.target_resolution.not_proven': 'Разрешение адреса не подтверждено.',
  'endpoint.checkMessage.target_resolution.passed': 'Адрес успешно разрешен.',
  'endpoint.checkMessage.tcp_connect.failed': 'TCP-подключение завершилось ошибкой.',
  'endpoint.checkMessage.tcp_connect.not_proven': 'TCP-подключение не подтверждено.',
  'endpoint.checkMessage.tcp_connect.passed': 'TCP-подключение успешно установлено.',
  'endpoint.checkMessage.tls_handshake.failed': 'TLS-рукопожатие завершилось ошибкой.',
  'endpoint.checkMessage.tls_handshake.not_proven': 'TLS-рукопожатие не подтверждено.',
  'endpoint.checkMessage.tls_handshake.passed': 'TLS-рукопожатие успешно выполнено.',
  'endpoint.checkMessage.tls_handshake.skipped': 'TLS-рукопожатие пропущено.',
  'endpoint.grpcBlocked': 'не доказано',
  'endpoint.grpcReady': 'готов',
  'endpoint.importPaths': 'Пути импорта (опционально, по одному в строке)',
  'endpoint.importPathsPlaceholder': '/absolute/path/to/import-root',
  'endpoint.loadProto': 'Загрузить proto-каталог',
  'endpoint.loadProtoPending': 'Загружаем proto-каталог...',
  'endpoint.loadReflection': 'Загрузить каталог reflection',
  'endpoint.loadReflectionPending': 'Загружаем reflection...',
  'endpoint.preflightEmpty': 'Запустите предварительную проверку эндпоинта, чтобы проверить транспорт, TLS и готовность gRPC.',
  'endpoint.preflightTitle': 'Проверка транспорта',
  'endpoint.protoDirectories': 'Proto-директории (по одной в строке)',
  'endpoint.protoDirectoriesPlaceholder': '/absolute/path/to/proto',
  'endpoint.protoFiles': 'Proto-файлы (опционально, по одному в строке)',
  'endpoint.protoFilesPlaceholder': '/absolute/path/to/service.proto',
  'endpoint.protoNote': 'Proto-каталог перезагружается только по кнопке ниже. Отслеживание файлов не входит в этот срез.',
  'endpoint.requestTimeout': 'Таймаут запроса (мс)',
  'endpoint.runPreflight': 'Запустить предварительную проверку',
  'endpoint.serverNameOverride': 'Переопределение имени сервера',
  'endpoint.serverNamePlaceholder': 'опциональное переопределение SAN',
  'endpoint.streamIdleTimeout': 'Таймаут простоя потока (мс)',
  'endpoint.target': 'Адрес',
  'endpoint.testPending': 'Проверяем эндпоинт...',
  'endpoint.title': 'Эндпоинт',
  'endpoint.transport': 'транспорт',
  'endpoint.tlsFailed': 'ошибка',
  'endpoint.tlsMode': 'TLS режим',
  'endpoint.tlsOff': 'выкл',
  'endpoint.tlsOk': 'готово',
  'endpoint.transportBlocked': 'заблокирован',
  'endpoint.transportReachable': 'доступен',

  'errors.addProtoSource': 'Добавьте хотя бы одну proto-директорию или файл перед загрузкой proto-каталога.',
  'errors.bootstrapUnexpected': 'Не удалось запустить оболочку приложения.',
  'errors.diagnosticsProbeUnexpected': 'Неожиданная ошибка проверки диагностики.',
  'errors.endpointPreflight': 'Предварительная проверка эндпоинта неожиданно завершилась ошибкой.',
  'errors.historyDetail': 'Не удалось загрузить детали истории.',
  'errors.metadataObject': 'Переопределения метаданных должны быть JSON-объектом со строковыми значениями.',
  'errors.metadataStringValue': 'Значение метаданных для "{key}" должно быть строкой.',
  'errors.metadataValidJson': 'Переопределения метаданных должны быть валидным JSON.',
  'errors.protoCatalog': 'Не удалось загрузить proto-каталог.',
  'errors.reflectionCatalog': 'Не удалось загрузить каталог reflection.',
  'errors.requestBodyJson': 'Тело запроса должно быть валидным JSON.',
  'errors.requestSave': 'Не удалось сохранить запрос.',
  'errors.requestSaveLoadedMethod': 'Выберите загруженный метод перед сохранением переиспользуемого запроса.',
  'errors.requestSaveUnaryOnly': 'Сохранение запросов в этом срезе подключено к унарному компоновщику.',
  'errors.clientStreamMessagesArray': 'Статическая последовательность клиентского потока должна быть JSON-массивом сообщений запроса.',
  'errors.clientStreamMessagesRequired': 'Добавьте хотя бы одно сообщение запроса в статическую последовательность клиентского потока.',
  'errors.clientStreamHalfClose': 'Не удалось закрыть локальную отправку клиентского потока.',
  'errors.clientStreamHalfCloseUnavailable': 'Сначала запустите открытый интерактивный клиентский поток.',
  'errors.clientStreamSend': 'Не удалось отправить сообщение клиентского потока.',
  'errors.clientStreamSendUnavailable': 'Сначала запустите открытый интерактивный клиентский поток.',
  'errors.clientStreamStart': 'Не удалось запустить клиентский поток.',
  'errors.bidiStreamHalfClose': 'Не удалось закрыть локальную отправку двунаправленного потока.',
  'errors.bidiStreamHalfCloseUnavailable': 'Сначала запустите открытый двунаправленный поток.',
  'errors.bidiStreamSend': 'Не удалось отправить сообщение двунаправленного потока.',
  'errors.bidiStreamSendUnavailable': 'Сначала запустите открытый двунаправленный поток.',
  'errors.bidiStreamStart': 'Не удалось запустить двунаправленный поток.',
  'errors.selectMethodInvoke': 'Выберите метод из загруженного каталога перед вызовом.',
  'errors.selectMethodStream': 'Выберите метод из загруженного каталога перед запуском потока.',
  'errors.streamCancel': 'Не удалось отменить поток.',
  'errors.streamStart': 'Не удалось запустить серверный поток.',
  'errors.streamUnaryOnly': 'Выберите потоковый метод, который соответствует выбранному сценарию.',
  'errors.unaryInvoke': 'Унарный вызов неожиданно завершился ошибкой.',
  'errors.unaryOnly': 'Эта поверхность вызова пока поддерживает только унарные методы. Выберите унарный метод из каталога.',
  'errors.workspaceCreate': 'Не удалось создать рабочую область.',
  'errors.workspaceOpen': 'Не удалось открыть рабочую область.',
  'errors.workspaceSave': 'Не удалось сохранить рабочую область.',
  'errors.workspaceValidate': 'Не удалось проверить рабочую область.',

  'footer.notRun': 'не запускался',
  'footer.probe': 'проверка',
  'footer.stream': 'поток',

  'history.artifacts': 'Артефакты',
  'history.detailTitle': 'Детали сохраненной истории',
  'history.empty': 'Завершенные вызовы появятся здесь вместе со сводками и артефактами лога сессии.',
  'history.loading': 'Загружаем сохраненный артефакт сессии...',
  'history.recentTitle': 'Недавняя история вызовов',
  'history.storedSummary': 'Сохраненная сводка',
  'history.structuredLogEvents': 'Структурные события лога',

  'home.appShellCopy':
    'Активны четыре области макета, frontend загружается из метаданных Go runtime без браузерных допущений.',
  'home.appShellTitle': 'Оболочка приложения',
  'home.contractDrift': 'Обнаружено расхождение контракта, запуск заблокирован проверкой.',
  'home.contractGuardTitle': 'Проверка контракта',
  'home.contractMatch': 'Идентификаторы frontend и backend совпадают с манифестом контракта v1.',
  'home.copy': 'Runtime Wails, оболочка Svelte и шина диагностических событий подключены перед следующими функциональными срезами.',
  'home.eyebrow': 'UX стабилизация',
  'home.openSession': 'Открыть сессию',
  'home.openWorkspace': 'Открыть рабочую область',
  'home.primaryFlow': 'Основной сценарий',
  'home.slices': 'Срезы Epic 0',
  'home.slice.0_1.summary': 'Оболочка Wails, каркас Svelte-приложения, runtime-привязка и полный цикл диагностического события.',
  'home.slice.0_2.summary': 'Манифест контракта, DTO вызовов и границы модулей закреплены в общих runtime-метаданных.',
  'home.slice.0_3.summary': 'Канонические состояния потоков, панели и таксономия ошибок подключены к общим путям кода.',
  'home.sliceStatus.implemented': 'реализовано',
  'home.title': 'Рабочая оболочка запущена',

  'method.catalogEmptyProto': 'Загрузите локальные proto-источники и пути импорта, чтобы построить дерево методов без server reflection.',
  'method.catalogEmptyReflection': 'Загрузите reflection, чтобы построить дерево сервисов, шаблоны запросов и RPC-метаданные для сценария вызова.',
  'method.catalogTitle': 'Каталог методов',
  'method.method': 'Метод',
  'method.requestType': 'Тип запроса',
  'method.responseType': 'Тип ответа',
  'method.rpc.bidi_stream': 'Двунаправленный поток',
  'method.rpc.client_stream': 'Клиентский поток',
  'method.rpc.server_stream': 'Серверный поток',
  'method.rpc.unary': 'Унарный',
  'method.types.request': 'запрос:',
  'method.types.response': 'ответ:',

  'nav.diagnosticsOverlay': 'Диагностика',
  'nav.diagnosticsOverlayCopy': 'События runtime и классифицированные ошибки.',
  'nav.historyOverlay': 'История',
  'nav.historyOverlayCopy': 'Сохраненные сводки вызовов.',
  'nav.homeCopy': 'Готовность и статус проекта.',
  'nav.homeTitle': 'Главная',
  'nav.overlays': 'Панели',
  'nav.primaryFlow': 'Основной сценарий',
  'nav.sessionCopy': 'Состояние потока, проверка и диагностика.',
  'nav.sessionTitle': 'Сессия',
  'nav.settingsOverlay': 'Настройки',
  'nav.settingsOverlayCopy': 'Язык и локальные настройки.',
  'nav.workspaceCopy': 'Эндпоинт, каталог, запрос и ответ.',
  'nav.workspaceTitle': 'Рабочая область',

  'request.bodyJson': 'JSON тела запроса',
  'request.bidiMessageJson': 'JSON сообщения bidi',
  'request.clientMessageJson': 'JSON сообщения клиента',
  'request.clientSequenceJson': 'JSON статической последовательности сообщений',
  'request.clientStreamMode': 'Режим клиентского потока',
  'request.clientStreamModeInteractive': 'Интерактивный',
  'request.clientStreamModeStatic': 'Статическая последовательность',
  'request.composerTitle': 'Компоновщик запроса',
  'request.empty': 'Выберите метод из загруженного каталога, чтобы получить стартовую JSON-нагрузку.',
  'request.invoke': 'Выполнить унарный вызов',
  'request.invokePending': 'Выполняем унарный вызов...',
  'request.metadataJson': 'JSON переопределений метаданных',
  'request.resetTemplate': 'Сбросить к шаблону',
  'request.saveRequest': 'Сохранить запрос',
  'request.saveRequestPending': 'Сохраняем запрос...',
  'request.savedRequestId': 'ID сохраненного запроса',
  'request.shortcut': 'Используйте {shortcut} в редакторах, чтобы запустить выбранный унарный или потоковый метод.',
  'request.startStream': 'Запустить серверный поток',
  'request.startStreamPending': 'Запускаем поток...',
  'request.startClientStream': 'Запустить клиентский поток',
  'request.startClientStreamPending': 'Выполняем клиентский поток...',
  'request.startInteractiveClientStream': 'Запустить интерактивный поток',
  'request.startBidiStream': 'Запустить bidi поток',
  'request.startBidiStreamPending': 'Запускаем bidi поток...',
  'request.sendClientMessage': 'Отправить сообщение',
  'request.sendClientMessagePending': 'Отправляем...',
  'request.sendBidiMessage': 'Отправить сообщение',
  'request.sendBidiMessagePending': 'Отправляем...',
  'request.halfClose': 'Закрыть отправку',
  'request.halfClosePending': 'Закрываем отправку...',
  'request.cancelStream': 'Отменить поток',
  'request.cancelStreamPending': 'Отменяем...',
  'request.unsupportedMethod': '{method}: тип {rpcType}.',
  'request.unsupported': 'Срез 3.4 поддерживает унарные, серверные, клиентские и двунаправленные потоковые методы.',

  'response.body': 'Тело ответа',
  'response.empty': 'Заголовки, статус, трейлеры, тело унарного ответа и лента потоковых событий появятся здесь после запуска вызова.',
  'response.finished': 'Завершен',
  'response.headers': 'Заголовки',
  'response.panelTitle': 'Панель ответа',
  'response.started': 'Начат',
  'response.status': 'Статус',
  'response.trailers': 'Трейлеры',

  'session.backToWorkspace': 'Вернуться к рабочей области',
  'session.copy': 'UI-оболочка и среда Go используют общие состояния потоков, терминальные исходы и таксономию диагностики.',
  'session.diagnosticsProbe': 'Проверка диагностики',
  'session.errorTaxonomy': 'Таксономия ошибок',
  'session.eyebrow': 'Slice 0.3',
  'session.lastAcknowledgement': 'Последнее подтверждение моста событий:',
  'session.noConditions': 'нет условий',
  'session.probeNotCompleted': 'Проверка диагностики еще не завершилась.',
  'session.recentDiagnostics': 'Недавняя диагностика',
  'session.title': 'Каноническая модель состояния сессии',
  'session.waitingEmission': 'Ждем отправку события среды.',

  'source.proto': 'proto-источники',
  'source.reflection': 'server reflection',

  'stream.cancelRequested': 'Запрошена отмена {callId}.',
  'stream.condition.truncated': 'усечено',
  'stream.completedClosed': '{rpcType} сохранен в истории как {callId}.',
  'stream.completedOther': '{rpcType} завершился как {state} со статусом {status}.',
  'stream.contextLocked': 'Отмените активный поток {callId}, прежде чем менять endpoint, каталог или метод.',
  'stream.contextLockedInfo': 'Активный поток {callId} закреплен здесь, пока он не завершится или не будет отменен.',
  'stream.emptyFilteredTimeline': 'Нет событий потока, подходящих под текущие фильтры ленты.',
  'stream.emptyTimeline': 'События потока появятся здесь по мере отправки сообщений и получения заголовков, ответов и трейлеров.',
  'stream.halfClosedLocalReceiving': 'Локальная отправка закрыта; входящие ответы еще могут приходить.',
  'stream.halfCloseRequested': 'Локальная отправка закрыта для {callId}.',
  'stream.messageSent': 'Сообщение клиента #{index} отправлено для {callId}.',
  'stream.started': '{rpcType} запущен как {callId}.',
  'stream.truncatedWarning': 'Сессия помечена как усеченная; видимая live-лента может быть неполной.',
  'streamState.cancelled': 'отменен',
  'streamState.closed': 'закрыт',
  'streamState.connecting': 'подключение',
  'streamState.error': 'ошибка',
  'streamState.half_closed_local': 'локальная отправка закрыта',
  'streamState.half_closed_remote': 'удаленная отправка закрыта',
  'streamState.idle': 'ожидание',
  'streamState.open': 'открыт',

  'tls.custom_ca': 'Пользовательский CA',
  'tls.mtls': 'mTLS',
  'tls.plaintext': 'Без TLS',
  'tls.system_ca': 'Системные CA',

  'timeline.allDirections': 'все',
  'timeline.allKinds': 'все',
  'timeline.direction': 'Направление',
  'timeline.eventsCount': '{count} событий',
  'timeline.jumpToError': 'К ошибке',
  'timeline.jumpToLive': 'К live-хвосту',
  'timeline.kind': 'Тип',
  'timeline.received': 'полученные',
  'timeline.sent': 'отправленные',
  'timeline.windowCount': 'отрисовано {count}/{total}',

  'topbar.contract': 'контракт',
  'topbar.eyebrow': 'UX / i18n стабилизация',
  'topbar.probe': 'Проверить мост событий',
  'topbar.probePending': 'Проверяем мост событий...',
  'topbar.runtime': 'среда',

  'workspace.created': 'Рабочая область {name} создана: {manifestPath}.{backup}',
  'workspace.backupSuffix': ' Резервная копия: {path}.',
  'workspace.fileTitle': 'Файл рабочей области',
  'workspace.id': 'ID рабочей области',
  'workspace.infoDiagnostics': 'Структурная диагностика попадет в панель диагностики, если среда сможет классифицировать ошибку.',
  'workspace.manifest': 'Манифест',
  'workspace.name': 'Имя',
  'workspace.namePlaceholder': 'имя рабочей области',
  'workspace.opened': 'Рабочая область {name} открыта: {manifestPath}.{backup}',
  'workspace.path': 'Путь рабочей области',
  'workspace.pathPlaceholder': '/absolute/path/to/workspace',
  'workspace.saved': 'Рабочая область {name} сохранена: {manifestPath}.{backup}',
  'workspace.validationIssues': 'Черновик рабочей области содержит ошибки проверки: {count}.',
  'workspace.validationPassed': 'Черновик рабочей области прошел проверку открытия и сохранения.',

  'workspaceView.copy': 'Server reflection, proto-каталоги, унарные вызовы и потоковые сценарии собраны в единую рабочую область.',
  'workspaceView.eyebrow': 'Slice 4.1',
  'workspaceView.title': 'Рабочая область gRPC',

  'call.unarySaved': 'Унарный вызов сохранен в истории как {callId}.',
  'call.unarySavedWithStatus': 'Унарный вызов завершился со статусом {status} и сохранен в истории как {callId}.',
  'call.requestSaved': 'Запрос {id} сохранен: {path}.',

  'view.home': 'Главная',
  'view.session': 'Сессия',
  'view.workspace': 'Рабочая область',
}

export const TRANSLATIONS: TranslationDictionaries = {
  en: EN_TRANSLATIONS,
  ru: RU_TRANSLATIONS,
}

export function isSupportedLanguage(value: string | null | undefined): value is Language {
  return SUPPORTED_LANGUAGES.some((language) => language === value)
}

function interpolate(template: string, params: TranslationParams | undefined): string {
  if (!params) {
    return template
  }

  return template.replace(/\{([a-zA-Z0-9_]+)\}/g, (match, key: string) => {
    const value = params[key]
    return value === null || value === undefined ? '' : String(value)
  })
}

export function translateFromDictionaries(
  language: Language,
  dictionaries: TranslationDictionaries,
  key: string,
  params?: TranslationParams,
): string {
  const template = dictionaries[language]?.[key] ?? dictionaries.en?.[key] ?? key
  return interpolate(template, params)
}

export function translate(language: Language, key: string, params?: TranslationParams): string {
  return translateFromDictionaries(language, TRANSLATIONS, key, params)
}

function stableKeyPart(value: string): string {
  return value
    .trim()
    .replace(/[^a-zA-Z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '')
    .toLowerCase()
}

function translateStable(
  language: Language,
  prefix: string,
  stableId: string,
  fallback: string,
  suffix?: string,
): string {
  const key = `${prefix}.${stableKeyPart(stableId)}${suffix ? `.${suffix}` : ''}`
  const translated = translate(language, key)
  return translated === key ? fallback : translated
}

export function translateProductLine(language: Language, fallback: string | undefined): string {
  const translated = translate(language, 'common.productLine')
  return translated === 'common.productLine' ? (fallback ?? translate(language, 'common.productLineFallback')) : translated
}

export function translateViewLabel(language: Language, view: string): string {
  return translateStable(language, 'view', view, view)
}

export function translateStreamStateLabel(language: Language, state: string): string {
  return translateStable(language, 'streamState', state, state)
}

export function translateBootstrapSliceSummary(language: Language, sliceId: string, fallback: string): string {
  return translateStable(language, 'home.slice', sliceId, fallback, 'summary')
}

export function translateBootstrapSliceStatus(language: Language, status: string): string {
  return translateStable(language, 'home.sliceStatus', status, status)
}

export function translateDiagnosticCodeLabel(language: Language, code: string): string {
  return translateStable(language, 'diagnostics.code', code, code, 'label')
}

export function translateDiagnosticMessage(language: Language, code: string, fallback: string): string {
  return translateStable(language, 'diagnostics.code', code, fallback, 'message')
}

export function translateDiagnosticNextStep(language: Language, code: string, fallback: string): string {
  return translateStable(language, 'diagnostics.code', code, fallback, 'nextStep')
}

export function translateEndpointCheckStage(language: Language, stage: string): string {
  return translateStable(language, 'endpoint.checkStage', stage, stage)
}

export function translateEndpointCheckOutcome(language: Language, outcome: string): string {
  return translateStable(language, 'endpoint.checkOutcome', outcome, outcome)
}

export function translateEndpointCheckMessage(
  language: Language,
  stage: string,
  outcome: string,
  fallback: string,
): string {
  const key = `endpoint.checkMessage.${stableKeyPart(stage)}.${stableKeyPart(outcome)}`
  const translated = translate(language, key)
  return translated === key ? fallback : translated
}

export interface LanguageStorage {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
}

function browserStorage(): LanguageStorage | undefined {
  if (typeof window === 'undefined') {
    return undefined
  }

  return window.localStorage
}

function browserLocale(): string {
  if (typeof navigator === 'undefined') {
    return ''
  }

  return navigator.language
}

export function detectDefaultLanguage(locale: string): Language {
  return locale.toLowerCase().startsWith('ru') ? 'ru' : 'en'
}

export function resolveInitialLanguage(options: {
  storage?: LanguageStorage
  locale?: string
} = {}): Language {
  const stored = options.storage?.getItem(LANGUAGE_STORAGE_KEY)
  if (isSupportedLanguage(stored)) {
    return stored
  }

  return detectDefaultLanguage(options.locale ?? browserLocale())
}

function createLanguageStore() {
  const { subscribe, set } = writable<Language>(
    resolveInitialLanguage({
      storage: browserStorage(),
    }),
  )

  return {
    subscribe,
    set(language: Language) {
      browserStorage()?.setItem(LANGUAGE_STORAGE_KEY, language)
      set(language)
    },
  }
}

export const language = createLanguageStore()

export function setLanguage(nextLanguage: Language): void {
  language.set(nextLanguage)
}

export const i18n = derived(language, ($language) => ({
  language: $language,
  t: (key: string, params?: TranslationParams) => translate($language, key, params),
}))
