// (c) 2019-2020, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package config

const (
	ConfigFileKey                             = "config-file"
	VersionKey                                = "version"
	GenesisConfigFileKey                      = "genesis"
	NetworkNameKey                            = "network-id"
	TxFeeKey                                  = "tx-fee"
	CreateAssetTxFeeKey                       = "create-asset-tx-fee"
	CreateSubnetTxFeeKey                      = "create-subnet-tx-fee"
	CreateBlockchainTxFeeKey                  = "create-blockchain-tx-fee"
	UptimeRequirementKey                      = "uptime-requirement"
	MinValidatorStakeKey                      = "min-validator-stake"
	MaxValidatorStakeKey                      = "max-validator-stake"
	MinDelegatorStakeKey                      = "min-delegator-stake"
	MinDelegatorFeeKey                        = "min-delegation-fee"
	MinStakeDurationKey                       = "min-stake-duration"
	MaxStakeDurationKey                       = "max-stake-duration"
	StakeMintingPeriodKey                     = "stake-minting-period"
	AssertionsEnabledKey                      = "assertions-enabled"
	SignatureVerificationEnabledKey           = "signature-verification-enabled"
	DBTypeKey                                 = "db-type"
	DBPathKey                                 = "db-dir"
	DBConfigFileKey                           = "db-config-file"
	PublicIPKey                               = "public-ip"
	DynamicUpdateDurationKey                  = "dynamic-update-duration"
	DynamicPublicIPResolverKey                = "dynamic-public-ip"
	InboundConnUpgradeThrottlerCooldownKey    = "inbound-connection-throttling-cooldown"
	InboundConnUpgradeThrottlerMaxRecentKey   = "inbound-connection-throttling-max-recent" // Deprecated starting in v1.6.0. TODO remove in a future release.
	InboundThrottlerMaxConnsPerSecKey         = "inbound-connection-throttling-max-conns-per-sec"
	OutboundConnectionThrottlingRps           = "outbound-connection-throttling-rps"
	OutboundConnectionTimeout                 = "outbound-connection-timeout"
	HTTPHostKey                               = "http-host"
	HTTPPortKey                               = "http-port"
	HTTPSEnabledKey                           = "http-tls-enabled"
	HTTPSKeyFileKey                           = "http-tls-key-file"
	HTTPSCertFileKey                          = "http-tls-cert-file"
	HTTPAllowedOrigins                        = "http-allowed-origins"
	APIAuthRequiredKey                        = "api-auth-required"
	APIAuthPasswordFileKey                    = "api-auth-password-file" // #nosec G101
	BootstrapIPsKey                           = "bootstrap-ips"
	BootstrapIDsKey                           = "bootstrap-ids"
	StakingPortKey                            = "staking-port"
	StakingEnabledKey                         = "staking-enabled"
	StakingEphemeralCertEnabledKey            = "staking-ephemeral-cert-enabled"
	StakingKeyPathKey                         = "staking-tls-key-file"
	StakingCertPathKey                        = "staking-tls-cert-file"
	StakingDisabledWeightKey                  = "staking-disabled-weight"
	NetworkInitialTimeoutKey                  = "network-initial-timeout"
	NetworkMinimumTimeoutKey                  = "network-minimum-timeout"
	NetworkMaximumTimeoutKey                  = "network-maximum-timeout"
	NetworkTimeoutHalflifeKey                 = "network-timeout-halflife"
	NetworkTimeoutCoefficientKey              = "network-timeout-coefficient"
	NetworkHealthMinPeersKey                  = "network-health-min-conn-peers"
	NetworkHealthMaxTimeSinceMsgReceivedKey   = "network-health-max-time-since-msg-received"
	NetworkHealthMaxTimeSinceMsgSentKey       = "network-health-max-time-since-msg-sent"
	NetworkHealthMaxPortionSendQueueFillKey   = "network-health-max-portion-send-queue-full"
	NetworkHealthMaxSendFailRateKey           = "network-health-max-send-fail-rate"
	NetworkHealthMaxOutstandingDurationKey    = "network-health-max-outstanding-request-duration"
	NetworkPeerListSizeKey                    = "network-peer-list-size"
	NetworkPeerListGossipSizeKey              = "network-peer-list-gossip-size"
	NetworkPeerListGossipFreqKey              = "network-peer-list-gossip-frequency"
	NetworkPeerListStakerGossipFractionKey    = "network-peer-list-staker-gossip-fraction"
	NetworkInitialReconnectDelayKey           = "network-initial-reconnect-delay"
	NetworkGetVersionTimeoutKey               = "network-get-version-timeout"
	NetworkReadHandshakeTimeoutKey            = "network-read-handshake-timeout"
	NetworkPingTimeoutKey                     = "network-ping-timeout"
	NetworkPingFrequencyKey                   = "network-ping-frequency"
	NetworkMaxReconnectDelayKey               = "network-max-reconnect-delay"
	NetworkCompressionEnabledKey              = "network-compression-enabled"
	NetworkMaxClockDifferenceKey              = "network-max-clock-difference"
	NetworkAllowPrivateIPsKey                 = "network-allow-private-ips"
	NetworkRequireValidatorToConnectKey       = "network-require-validator-to-connect"
	BenchlistFailThresholdKey                 = "benchlist-fail-threshold"
	BenchlistPeerSummaryEnabledKey            = "benchlist-peer-summary-enabled"
	BenchlistDurationKey                      = "benchlist-duration"
	BenchlistMinFailingDurationKey            = "benchlist-min-failing-duration"
	BuildDirKey                               = "build-dir"
	LogsDirKey                                = "log-dir"
	LogLevelKey                               = "log-level"
	LogDisplayLevelKey                        = "log-display-level"
	LogDisplayHighlightKey                    = "log-display-highlight"
	SnowSampleSizeKey                         = "snow-sample-size"
	SnowQuorumSizeKey                         = "snow-quorum-size"
	SnowVirtuousCommitThresholdKey            = "snow-virtuous-commit-threshold"
	SnowRogueCommitThresholdKey               = "snow-rogue-commit-threshold"
	SnowAvalancheNumParentsKey                = "snow-avalanche-num-parents"
	SnowAvalancheBatchSizeKey                 = "snow-avalanche-batch-size"
	SnowConcurrentRepollsKey                  = "snow-concurrent-repolls"
	SnowOptimalProcessingKey                  = "snow-optimal-processing"
	SnowMaxProcessingKey                      = "snow-max-processing"
	SnowMaxTimeProcessingKey                  = "snow-max-time-processing"
	SnowEpochFirstTransitionKey               = "snow-epoch-first-transition"
	SnowEpochDurationKey                      = "snow-epoch-duration"
	WhitelistedSubnetsKey                     = "whitelisted-subnets"
	AdminAPIEnabledKey                        = "api-admin-enabled"
	InfoAPIEnabledKey                         = "api-info-enabled"
	KeystoreAPIEnabledKey                     = "api-keystore-enabled"
	MetricsAPIEnabledKey                      = "api-metrics-enabled"
	HealthAPIEnabledKey                       = "api-health-enabled"
	IpcAPIEnabledKey                          = "api-ipcs-enabled"
	IpcsChainIDsKey                           = "ipcs-chain-ids"
	IpcsPathKey                               = "ipcs-path"
	MeterVMsEnabledKey                        = "meter-vms-enabled"
	ConsensusGossipFrequencyKey               = "consensus-gossip-frequency"
	ConsensusGossipAcceptedFrontierSizeKey    = "consensus-accepted-frontier-gossip-size"
	ConsensusGossipOnAcceptSizeKey            = "consensus-on-accept-gossip-size"
	AppGossipNonValidatorSizeKey              = "consensus-app-gossip-non-validator-size"
	AppGossipValidatorSizeKey                 = "consensus-app-gossip-validator-size"
	ConsensusShutdownTimeoutKey               = "consensus-shutdown-timeout"
	FdLimitKey                                = "fd-limit"
	IndexEnabledKey                           = "index-enabled"
	IndexAllowIncompleteKey                   = "index-allow-incomplete"
	RouterHealthMaxDropRateKey                = "router-health-max-drop-rate"
	RouterHealthMaxOutstandingRequestsKey     = "router-health-max-outstanding-requests"
	HealthCheckFreqKey                        = "health-check-frequency"
	HealthCheckAveragerHalflifeKey            = "health-check-averager-halflife"
	RetryBootstrapKey                         = "bootstrap-retry-enabled"
	RetryBootstrapWarnFrequencyKey            = "bootstrap-retry-warn-frequency"
	PeerAliasTimeoutKey                       = "peer-alias-timeout"
	PluginModeKey                             = "plugin-mode-enabled"
	BootstrapBeaconConnectionTimeoutKey       = "bootstrap-beacon-connection-timeout"
	BootstrapMaxTimeGetAncestorsKey           = "boostrap-max-time-get-ancestors"
	BootstrapMultiputMaxContainersSentKey     = "bootstrap-multiput-max-containers-sent"
	BootstrapMultiputMaxContainersReceivedKey = "bootstrap-multiput-max-containers-received"
	ChainConfigDirKey                         = "chain-config-dir"
	SubnetConfigDirKey                        = "subnet-config-dir"
	ProfileDirKey                             = "profile-dir"
	ProfileContinuousEnabledKey               = "profile-continuous-enabled"
	ProfileContinuousFreqKey                  = "profile-continuous-freq"
	ProfileContinuousMaxFilesKey              = "profile-continuous-max-files"
	InboundThrottlerAtLargeAllocSizeKey       = "throttler-inbound-at-large-alloc-size"
	InboundThrottlerVdrAllocSizeKey           = "throttler-inbound-validator-alloc-size"
	InboundThrottlerNodeMaxAtLargeBytesKey    = "throttler-inbound-node-max-at-large-bytes"
	OutboundThrottlerAtLargeAllocSizeKey      = "throttler-outbound-at-large-alloc-size"
	OutboundThrottlerVdrAllocSizeKey          = "throttler-outbound-validator-alloc-size"
	OutboundThrottlerNodeMaxAtLargeBytesKey   = "throttler-outbound-node-max-at-large-bytes"
	VMAliasesFileKey                          = "vm-aliases-file"
)
