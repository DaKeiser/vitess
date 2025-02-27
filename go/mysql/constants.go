/*
Copyright 2019 The Vitess Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mysql

import (
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/simplifiedchinese"
)

const (
	// MaxPacketSize is the maximum payload length of a packet
	// the server supports.
	MaxPacketSize = (1 << 24) - 1

	// protocolVersion is the current version of the protocol.
	// Always 10.
	protocolVersion = 10
)

// AuthMethodDescription is the type for different supported and
// implemented authentication methods.
type AuthMethodDescription string

// Supported auth forms.
const (
	// MysqlNativePassword uses a salt and transmits a hash on the wire.
	MysqlNativePassword = AuthMethodDescription("mysql_native_password")

	// MysqlClearPassword transmits the password in the clear.
	MysqlClearPassword = AuthMethodDescription("mysql_clear_password")

	// CachingSha2Password uses a salt and transmits a SHA256 hash on the wire.
	CachingSha2Password = AuthMethodDescription("caching_sha2_password")

	// MysqlDialog uses the dialog plugin on the client side.
	// It transmits data in the clear.
	MysqlDialog = AuthMethodDescription("dialog")
)

// Capability flags.
// Originally found in include/mysql/mysql_com.h
const (
	// CapabilityClientLongPassword is CLIENT_LONG_PASSWORD.
	// New more secure passwords. Assumed to be set since 4.1.1.
	// We do not check this anywhere.
	CapabilityClientLongPassword = 1

	// CapabilityClientFoundRows is CLIENT_FOUND_ROWS.
	CapabilityClientFoundRows = 1 << 1

	// CapabilityClientLongFlag is CLIENT_LONG_FLAG.
	// Longer flags in Protocol::ColumnDefinition320.
	// Set it everywhere, not used, as we use Protocol::ColumnDefinition41.
	CapabilityClientLongFlag = 1 << 2

	// CapabilityClientConnectWithDB is CLIENT_CONNECT_WITH_DB.
	// One can specify db on connect.
	CapabilityClientConnectWithDB = 1 << 3

	// CLIENT_NO_SCHEMA 1 << 4
	// Do not permit database.table.column. We do permit it.

	// CLIENT_COMPRESS 1 << 5
	// We do not support compression. CPU is usually our bottleneck.

	// CLIENT_ODBC 1 << 6
	// No special behavior since 3.22.

	// CLIENT_LOCAL_FILES 1 << 7
	// Client can use LOCAL INFILE request of LOAD DATA|XML.
	// We do not set it.

	// CLIENT_IGNORE_SPACE 1 << 8
	// Parser can ignore spaces before '('.
	// We ignore this.

	// CapabilityClientProtocol41 is CLIENT_PROTOCOL_41.
	// New 4.1 protocol. Enforced everywhere.
	CapabilityClientProtocol41 = 1 << 9

	// CLIENT_INTERACTIVE 1 << 10
	// Not specified, ignored.

	// CapabilityClientSSL is CLIENT_SSL.
	// Switch to SSL after handshake.
	CapabilityClientSSL = 1 << 11

	// CLIENT_IGNORE_SIGPIPE 1 << 12
	// Do not issue SIGPIPE if network failures occur (libmysqlclient only).

	// CapabilityClientTransactions is CLIENT_TRANSACTIONS.
	// Can send status flags in EOF_Packet.
	// This flag is optional in 3.23, but always set by the server since 4.0.
	// We just do it all the time.
	CapabilityClientTransactions = 1 << 13

	// CLIENT_RESERVED 1 << 14

	// CapabilityClientSecureConnection is CLIENT_SECURE_CONNECTION.
	// New 4.1 authentication. Always set, expected, never checked.
	CapabilityClientSecureConnection = 1 << 15

	// CapabilityClientMultiStatements is CLIENT_MULTI_STATEMENTS
	// Can handle multiple statements per COM_QUERY and COM_STMT_PREPARE.
	CapabilityClientMultiStatements = 1 << 16

	// CapabilityClientMultiResults is CLIENT_MULTI_RESULTS
	// Can send multiple resultsets for COM_QUERY.
	CapabilityClientMultiResults = 1 << 17

	// CapabilityClientPluginAuth is CLIENT_PLUGIN_AUTH.
	// Client supports plugin authentication.
	CapabilityClientPluginAuth = 1 << 19

	// CapabilityClientConnAttr is CLIENT_CONNECT_ATTRS
	// Permits connection attributes in Protocol::HandshakeResponse41.
	CapabilityClientConnAttr = 1 << 20

	// CapabilityClientPluginAuthLenencClientData is CLIENT_PLUGIN_AUTH_LENENC_CLIENT_DATA
	CapabilityClientPluginAuthLenencClientData = 1 << 21

	// CLIENT_CAN_HANDLE_EXPIRED_PASSWORDS 1 << 22
	// Announces support for expired password extension.
	// Not yet supported.

	// CLIENT_SESSION_TRACK 1 << 23
	// Can set ServerSessionStateChanged in the Status Flags
	// and send session-state change data after a OK packet.
	// Not yet supported.
	CapabilityClientSessionTrack = 1 << 23

	// CapabilityClientDeprecateEOF is CLIENT_DEPRECATE_EOF
	// Expects an OK (instead of EOF) after the resultset rows of a Text Resultset.
	CapabilityClientDeprecateEOF = 1 << 24
)

// Status flags. They are returned by the server in a few cases.
// Originally found in include/mysql/mysql_com.h
// See http://dev.mysql.com/doc/internals/en/status-flags.html
const (
	// a transaction is active
	ServerStatusInTrans   uint16 = 0x0001
	NoServerStatusInTrans uint16 = 0xFFFE

	// auto-commit is enabled
	ServerStatusAutocommit   uint16 = 0x0002
	NoServerStatusAutocommit uint16 = 0xFFFD

	ServerMoreResultsExists     uint16 = 0x0008
	ServerStatusNoGoodIndexUsed uint16 = 0x0010
	ServerStatusNoIndexUsed     uint16 = 0x0020
	// Used by Binary Protocol Resultset to signal that COM_STMT_FETCH must be used to fetch the row-data.
	ServerStatusCursorExists       uint16 = 0x0040
	ServerStatusLastRowSent        uint16 = 0x0080
	ServerStatusDbDropped          uint16 = 0x0100
	ServerStatusNoBackslashEscapes uint16 = 0x0200
	ServerStatusMetadataChanged    uint16 = 0x0400
	ServerQueryWasSlow             uint16 = 0x0800
	ServerPsOutParams              uint16 = 0x1000
	// in a read-only transaction
	ServerStatusInTransReadonly uint16 = 0x2000
	// connection state information has changed
	ServerSessionStateChanged uint16 = 0x4000
)

// State Change Information
const (
	// one or more system variables changed.
	SessionTrackSystemVariables uint8 = 0x00
	// schema changed.
	SessionTrackSchema uint8 = 0x01
	// "track state change" changed.
	SessionTrackStateChange uint8 = 0x02
	// "track GTIDs" changed.
	SessionTrackGtids uint8 = 0x03
)

// Packet types.
// Originally found in include/mysql/mysql_com.h
const (
	// ComQuit is COM_QUIT.
	ComQuit = 0x01

	// ComInitDB is COM_INIT_DB.
	ComInitDB = 0x02

	// ComQuery is COM_QUERY.
	ComQuery = 0x03

	// ComFieldList is COM_Field_List.
	ComFieldList = 0x04

	// ComPing is COM_PING.
	ComPing = 0x0e

	// ComBinlogDump is COM_BINLOG_DUMP.
	ComBinlogDump = 0x12

	// ComSemiSyncAck is SEMI_SYNC_ACK.
	ComSemiSyncAck = 0xef

	// ComPrepare is COM_PREPARE.
	ComPrepare = 0x16

	// ComStmtExecute is COM_STMT_EXECUTE.
	ComStmtExecute = 0x17

	// ComStmtSendLongData is COM_STMT_SEND_LONG_DATA
	ComStmtSendLongData = 0x18

	// ComStmtClose is COM_STMT_CLOSE.
	ComStmtClose = 0x19

	// ComStmtReset is COM_STMT_RESET
	ComStmtReset = 0x1a

	//ComStmtFetch is COM_STMT_FETCH
	ComStmtFetch = 0x1c

	// ComSetOption is COM_SET_OPTION
	ComSetOption = 0x1b

	// ComResetConnection is COM_RESET_CONNECTION
	ComResetConnection = 0x1f

	// ComBinlogDumpGTID is COM_BINLOG_DUMP_GTID.
	ComBinlogDumpGTID = 0x1e

	// OKPacket is the header of the OK packet.
	OKPacket = 0x00

	// EOFPacket is the header of the EOF packet.
	EOFPacket = 0xfe

	// ErrPacket is the header of the error packet.
	ErrPacket = 0xff

	// NullValue is the encoded value of NULL.
	NullValue = 0xfb
)

// Auth packet types
const (
	// AuthMoreDataPacket is sent when server requires more data to authenticate
	AuthMoreDataPacket = 0x01

	// CachingSha2FastAuth is sent before OKPacket when server authenticates using cache
	CachingSha2FastAuth = 0x03

	// CachingSha2FullAuth is sent when server requests un-scrambled password to authenticate
	CachingSha2FullAuth = 0x04

	// AuthSwitchRequestPacket is used to switch auth method.
	AuthSwitchRequestPacket = 0xfe
)

// Error codes for client-side errors.
// Originally found in include/mysql/errmsg.h and
// https://dev.mysql.com/doc/refman/5.7/en/error-messages-client.html
const (
	// CRUnknownError is CR_UNKNOWN_ERROR
	CRUnknownError = 2000

	// CRConnectionError is CR_CONNECTION_ERROR
	// This is returned if a connection via a Unix socket fails.
	CRConnectionError = 2002

	// CRConnHostError is CR_CONN_HOST_ERROR
	// This is returned if a connection via a TCP socket fails.
	CRConnHostError = 2003

	// CRServerGone is CR_SERVER_GONE_ERROR.
	// This is returned if the client tries to send a command but it fails.
	CRServerGone = 2006

	// CRVersionError is CR_VERSION_ERROR
	// This is returned if the server versions don't match what we support.
	CRVersionError = 2007

	// CRServerHandshakeErr is CR_SERVER_HANDSHAKE_ERR
	CRServerHandshakeErr = 2012

	// CRServerLost is CR_SERVER_LOST.
	// Used when:
	// - the client cannot write an initial auth packet.
	// - the client cannot read an initial auth packet.
	// - the client cannot read a response from the server.
	//     This happens when a running query is killed.
	CRServerLost = 2013

	// CRCommandsOutOfSync is CR_COMMANDS_OUT_OF_SYNC
	// Sent when the streaming calls are not done in the right order.
	CRCommandsOutOfSync = 2014

	// CRNamedPipeStateError is CR_NAMEDPIPESETSTATE_ERROR.
	// This is the highest possible number for a connection error.
	CRNamedPipeStateError = 2018

	// CRCantReadCharset is CR_CANT_READ_CHARSET
	CRCantReadCharset = 2019

	// CRSSLConnectionError is CR_SSL_CONNECTION_ERROR
	CRSSLConnectionError = 2026

	// CRMalformedPacket is CR_MALFORMED_PACKET
	CRMalformedPacket = 2027
)

// Error codes for server-side errors.
// Originally found in include/mysql/mysqld_error.h and
// https://dev.mysql.com/doc/refman/5.7/en/error-messages-server.html
// The below are in sorted order by value, grouped by vterror code they should be bucketed into.
// See above reference for more information on each code.
const (
	// Vitess specific errors, (100-999)
	ERNotReplica = 100

	// unknown
	ERUnknownError = 1105

	// internal
	ERInternalError = 1815

	// unimplemented
	ERNotSupportedYet = 1235
	ERUnsupportedPS   = 1295

	// resource exhausted
	ERDiskFull               = 1021
	EROutOfMemory            = 1037
	EROutOfSortMemory        = 1038
	ERConCount               = 1040
	EROutOfResources         = 1041
	ERRecordFileFull         = 1114
	ERHostIsBlocked          = 1129
	ERCantCreateThread       = 1135
	ERTooManyDelayedThreads  = 1151
	ERNetPacketTooLarge      = 1153
	ERTooManyUserConnections = 1203
	ERLockTableFull          = 1206
	ERUserLimitReached       = 1226

	// deadline exceeded
	ERLockWaitTimeout = 1205

	// unavailable
	ERServerShutdown = 1053

	// not found
	ERCantFindFile          = 1017
	ERFormNotFound          = 1029
	ERKeyNotFound           = 1032
	ERBadFieldError         = 1054
	ERNoSuchThread          = 1094
	ERUnknownTable          = 1109
	ERCantFindUDF           = 1122
	ERNonExistingGrant      = 1141
	ERNoSuchTable           = 1146
	ERNonExistingTableGrant = 1147
	ERKeyDoesNotExist       = 1176
	ERDbDropExists          = 1008

	// permissions
	ERDBAccessDenied            = 1044
	ERAccessDeniedError         = 1045
	ERKillDenied                = 1095
	ERNoPermissionToCreateUsers = 1211
	ERSpecifiedAccessDenied     = 1227

	// failed precondition
	ERNoDb                          = 1046
	ERNoSuchIndex                   = 1082
	ERCantDropFieldOrKey            = 1091
	ERTableNotLockedForWrite        = 1099
	ERTableNotLocked                = 1100
	ERTooBigSelect                  = 1104
	ERNotAllowedCommand             = 1148
	ERTooLongString                 = 1162
	ERDelayedInsertTableLocked      = 1165
	ERDupUnique                     = 1169
	ERRequiresPrimaryKey            = 1173
	ERCantDoThisDuringAnTransaction = 1179
	ERReadOnlyTransaction           = 1207
	ERCannotAddForeign              = 1215
	ERNoReferencedRow               = 1216
	ERRowIsReferenced               = 1217
	ERCantUpdateWithReadLock        = 1223
	ERNoDefault                     = 1230
	EROperandColumns                = 1241
	ERSubqueryNo1Row                = 1242
	ERWarnDataOutOfRange            = 1264
	ERNonUpdateableTable            = 1288
	ERFeatureDisabled               = 1289
	EROptionPreventsStatement       = 1290
	ERDuplicatedValueInType         = 1291
	ERSPDoesNotExist                = 1305
	ERRowIsReferenced2              = 1451
	ErNoReferencedRow2              = 1452
	ErSPNotVarArg                   = 1414
	ERInnodbReadOnly                = 1874
	ERMasterFatalReadingBinlog      = 1236
	ERNoDefaultForField             = 1364

	// already exists
	ERTableExists    = 1050
	ERDupEntry       = 1062
	ERFileExists     = 1086
	ERUDFExists      = 1125
	ERDbCreateExists = 1007

	// aborted
	ERGotSignal          = 1078
	ERForcingClose       = 1080
	ERAbortingConnection = 1152
	ERLockDeadlock       = 1213

	// invalid arg
	ERUnknownComError              = 1047
	ERBadNullError                 = 1048
	ERBadDb                        = 1049
	ERBadTable                     = 1051
	ERNonUniq                      = 1052
	ERWrongFieldWithGroup          = 1055
	ERWrongGroupField              = 1056
	ERWrongSumSelect               = 1057
	ERWrongValueCount              = 1058
	ERTooLongIdent                 = 1059
	ERDupFieldName                 = 1060
	ERDupKeyName                   = 1061
	ERWrongFieldSpec               = 1063
	ERParseError                   = 1064
	EREmptyQuery                   = 1065
	ERNonUniqTable                 = 1066
	ERInvalidDefault               = 1067
	ERMultiplePriKey               = 1068
	ERTooManyKeys                  = 1069
	ERTooManyKeyParts              = 1070
	ERTooLongKey                   = 1071
	ERKeyColumnDoesNotExist        = 1072
	ERBlobUsedAsKey                = 1073
	ERTooBigFieldLength            = 1074
	ERWrongAutoKey                 = 1075
	ERWrongFieldTerminators        = 1083
	ERBlobsAndNoTerminated         = 1084
	ERTextFileNotReadable          = 1085
	ERWrongSubKey                  = 1089
	ERCantRemoveAllFields          = 1090
	ERUpdateTableUsed              = 1093
	ERNoTablesUsed                 = 1096
	ERTooBigSet                    = 1097
	ERBlobCantHaveDefault          = 1101
	ERWrongDbName                  = 1102
	ERWrongTableName               = 1103
	ERUnknownProcedure             = 1106
	ERWrongParamCountToProcedure   = 1107
	ERWrongParametersToProcedure   = 1108
	ERFieldSpecifiedTwice          = 1110
	ERInvalidGroupFuncUse          = 1111
	ERTableMustHaveColumns         = 1113
	ERUnknownCharacterSet          = 1115
	ERTooManyTables                = 1116
	ERTooManyFields                = 1117
	ERTooBigRowSize                = 1118
	ERWrongOuterJoin               = 1120
	ERNullColumnInIndex            = 1121
	ERFunctionNotDefined           = 1128
	ERWrongValueCountOnRow         = 1136
	ERInvalidUseOfNull             = 1138
	ERRegexpError                  = 1139
	ERMixOfGroupFuncAndFields      = 1140
	ERIllegalGrantForTable         = 1144
	ERSyntaxError                  = 1149
	ERWrongColumnName              = 1166
	ERWrongKeyColumn               = 1167
	ERBlobKeyWithoutLength         = 1170
	ERPrimaryCantHaveNull          = 1171
	ERTooManyRows                  = 1172
	ERLockOrActiveTransaction      = 1192
	ERUnknownSystemVariable        = 1193
	ERSetConstantsOnly             = 1204
	ERWrongArguments               = 1210
	ERWrongUsage                   = 1221
	ERWrongNumberOfColumnsInSelect = 1222
	ERDupArgument                  = 1225
	ERLocalVariable                = 1228
	ERGlobalVariable               = 1229
	ERWrongValueForVar             = 1231
	ERWrongTypeForVar              = 1232
	ERVarCantBeRead                = 1233
	ERCantUseOptionHere            = 1234
	ERIncorrectGlobalLocalVar      = 1238
	ERWrongFKDef                   = 1239
	ERKeyRefDoNotMatchTableRef     = 1240
	ERCyclicReference              = 1245
	ERCollationCharsetMismatch     = 1253
	ERCantAggregate2Collations     = 1267
	ERCantAggregate3Collations     = 1270
	ERCantAggregateNCollations     = 1271
	ERVariableIsNotStruct          = 1272
	ERUnknownCollation             = 1273
	ERWrongNameForIndex            = 1280
	ERWrongNameForCatalog          = 1281
	ERBadFTColumn                  = 1283
	ERTruncatedWrongValue          = 1292
	ERTooMuchAutoTimestampCols     = 1293
	ERInvalidOnUpdate              = 1294
	ERUnknownTimeZone              = 1298
	ERInvalidCharacterString       = 1300
	ERIllegalReference             = 1247
	ERDerivedMustHaveAlias         = 1248
	ERTableNameNotAllowedHere      = 1250
	ERQueryInterrupted             = 1317
	ERTruncatedWrongValueForField  = 1366
	ERIllegalValueForType          = 1367
	ERDataTooLong                  = 1406
	ErrWrongValueForType           = 1411
	ERWarnDataTruncated            = 1265
	ERForbidSchemaChange           = 1450
	ERDataOutOfRange               = 1690
	ERInvalidJSONText              = 3140
	ERInvalidJSONTextInParams      = 3141
	ERInvalidJSONBinaryData        = 3142
	ERInvalidJSONCharset           = 3144
	ERInvalidCastToJSON            = 3147
	ERJSONValueTooBig              = 3150
	ERJSONDocumentTooDeep          = 3157

	ErrCantCreateGeometryObject      = 1416
	ErrGISDataWrongEndianess         = 3055
	ErrNotImplementedForCartesianSRS = 3704
	ErrNotImplementedForProjectedSRS = 3705
	ErrNonPositiveRadius             = 3706

	// server not available
	ERServerIsntAvailable = 3168
)

// Sql states for errors.
// Originally found in include/mysql/sql_state.h
const (
	// SSUnknownSqlstate is ER_SIGNAL_EXCEPTION in
	// include/mysql/sql_state.h, but:
	// const char *unknown_sqlstate= "HY000"
	// in client.c. So using that one.
	SSUnknownSQLState = "HY000"

	// SSNetError is network related error
	SSNetError = "08S01"

	// SSWrongNumberOfColumns is related to columns error
	SSWrongNumberOfColumns = "21000"

	// SSWrongValueCountOnRow is related to columns count mismatch error
	SSWrongValueCountOnRow = "21S01"

	// SSDataTooLong is ER_DATA_TOO_LONG
	SSDataTooLong = "22001"

	// SSDataOutOfRange is ER_DATA_OUT_OF_RANGE
	SSDataOutOfRange = "22003"

	// SSConstraintViolation is constraint violation
	SSConstraintViolation = "23000"

	// SSCantDoThisDuringAnTransaction is
	// ER_CANT_DO_THIS_DURING_AN_TRANSACTION
	SSCantDoThisDuringAnTransaction = "25000"

	// SSAccessDeniedError is ER_ACCESS_DENIED_ERROR
	SSAccessDeniedError = "28000"

	// SSNoDB is ER_NO_DB_ERROR
	SSNoDB = "3D000"

	// SSLockDeadlock is ER_LOCK_DEADLOCK
	SSLockDeadlock = "40001"

	// SSClientError is the state on client errors
	SSClientError = "42000"

	// SSDupFieldName is ER_DUP_FIELD_NAME
	SSDupFieldName = "42S21"

	// SSBadFieldError is ER_BAD_FIELD_ERROR
	SSBadFieldError = "42S22"

	// SSUnknownTable is ER_UNKNOWN_TABLE
	SSUnknownTable = "42S02"

	// SSQueryInterrupted is ER_QUERY_INTERRUPTED;
	SSQueryInterrupted = "70100"
)

// CharacterSetEncoding maps a charset name to a golang encoder.
// golang does not support encoders for all MySQL charsets.
// A charset not in this map is unsupported.
// A trivial encoding (e.g. utf8) has a `nil` encoder
var CharacterSetEncoding = map[string]encoding.Encoding{
	"cp850":   charmap.CodePage850,
	"koi8r":   charmap.KOI8R,
	"latin1":  charmap.Windows1252,
	"latin2":  charmap.ISO8859_2,
	"ascii":   nil,
	"hebrew":  charmap.ISO8859_8,
	"greek":   charmap.ISO8859_7,
	"cp1250":  charmap.Windows1250,
	"gbk":     simplifiedchinese.GBK,
	"latin5":  charmap.ISO8859_9,
	"utf8":    nil,
	"utf8mb3": nil,
	"cp866":   charmap.CodePage866,
	"cp852":   charmap.CodePage852,
	"latin7":  charmap.ISO8859_13,
	"utf8mb4": nil,
	"cp1251":  charmap.Windows1251,
	"cp1256":  charmap.Windows1256,
	"cp1257":  charmap.Windows1257,
	"binary":  nil,
}

// IsNum returns true if a MySQL type is a numeric value.
// It is the same as IS_NUM defined in mysql.h.
func IsNum(typ uint8) bool {
	return (typ <= TypeInt24 && typ != TypeTimestamp) ||
		typ == TypeYear ||
		typ == TypeNewDecimal
}

// IsConnErr returns true if the error is a connection error.
func IsConnErr(err error) bool {
	if IsTooManyConnectionsErr(err) {
		return false
	}
	if sqlErr, ok := err.(*SQLError); ok {
		num := sqlErr.Number()
		return (num >= CRUnknownError && num <= CRNamedPipeStateError) || num == ERQueryInterrupted
	}
	return false
}

// IsConnLostDuringQuery returns true if the error is a CRServerLost error.
// Happens most commonly when a query is killed MySQL server-side.
func IsConnLostDuringQuery(err error) bool {
	if sqlErr, ok := err.(*SQLError); ok {
		num := sqlErr.Number()
		return (num == CRServerLost)
	}
	return false
}

// IsEphemeralError returns true if the error is ephemeral and the caller should
// retry if possible. Note: non-SQL errors are always treated as ephemeral.
func IsEphemeralError(err error) bool {
	if sqlErr, ok := err.(*SQLError); ok {
		en := sqlErr.Number()
		switch en {
		case
			CRConnectionError,
			CRConnHostError,
			CRMalformedPacket,
			CRNamedPipeStateError,
			CRServerLost,
			CRSSLConnectionError,
			ERCantCreateThread,
			ERDiskFull,
			ERForcingClose,
			ERGotSignal,
			ERHostIsBlocked,
			ERLockTableFull,
			ERInnodbReadOnly,
			ERInternalError,
			ERLockDeadlock,
			ERLockWaitTimeout,
			EROutOfMemory,
			EROutOfResources,
			EROutOfSortMemory,
			ERQueryInterrupted,
			ERServerIsntAvailable,
			ERServerShutdown,
			ERTooManyUserConnections,
			ERUnknownError,
			ERUserLimitReached:
			return true
		default:
			return false
		}
	}
	// If it's not an sqlError then we assume it's ephemeral
	return true
}

// IsTooManyConnectionsErr returns true if the error is due to too many connections.
func IsTooManyConnectionsErr(err error) bool {
	if sqlErr, ok := err.(*SQLError); ok {
		if sqlErr.Number() == CRServerHandshakeErr && strings.Contains(sqlErr.Message, "Too many connections") {
			return true
		}
	}
	return false
}

// IsSchemaApplyError returns true when given error is a MySQL error applying schema change
func IsSchemaApplyError(err error) bool {
	merr, isSQLErr := err.(*SQLError)
	if !isSQLErr {
		return false
	}
	switch merr.Num {
	case
		ERDupKeyName,
		ERCantDropFieldOrKey,
		ERTableExists,
		ERDupFieldName:
		return true
	}
	return false
}

type ReplicationState int

const (
	ReplicationStateUnknown ReplicationState = iota
	ReplicationStateStopped
	ReplicationStateConnecting
	ReplicationStateRunning
)

// ReplicationStatusToState converts a value you have for the IO thread(s) or SQL
// thread(s) or Group Replication applier thread(s) from MySQL or intermediate
// layers to a mysql.ReplicationState.
// on,yes,true == ReplicationStateRunning
// off,no,false == ReplicationStateStopped
// connecting == ReplicationStateConnecting
// anything else == ReplicationStateUnknown
func ReplicationStatusToState(s string) ReplicationState {
	// Group Replication uses ON instead of Yes
	switch strings.ToLower(s) {
	case "yes", "on", "true":
		return ReplicationStateRunning
	case "no", "off", "false":
		return ReplicationStateStopped
	case "connecting":
		return ReplicationStateConnecting
	default:
		return ReplicationStateUnknown
	}
}
