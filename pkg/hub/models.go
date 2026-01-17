package hub

import "time"

type ServiceStatus struct {
	Initialized bool      `json:"initialized"`
	Version     string    `json:"version"`
	Timestamp   time.Time `json:"timestamp"`
}

type ReadyStatus struct {
	Initialized bool   `json:"initialized"`
	Version     string `json:"version"`
	Refreshed   bool   `json:"refreshed,omitempty"`
}

type CacheResult struct {
	Success     bool      `json:"success"`
	Version     string    `json:"version"`
	KeysCleared int       `json:"keysCleared"`
	Patterns    []string  `json:"patterns,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

type BaseQuest struct {
	Objectives map[string]interface{} `json:"objectives"`
	Rewards    map[string]interface{} `json:"rewards"`
	Count      int                    `json:"count"`
}

type AthenaChallengeBundle struct {
	TemplateID              string                   `json:"templateId"`
	ChallengeBundleSchedule string                   `json:"challengeBundleSchedule"`
	Objects                 []ChallengeBundleObject  `json:"objects"`
	Amount                  int                      `json:"amount"`
	Rarity                  string                   `json:"rarity"`
	CompletionRewards       []BundleCompletionReward `json:"completionRewards"`
}

type ChallengeBundleObject struct {
	QuestDefinition string                     `json:"questDefinition"`
	Rarity          string                     `json:"rarity"`
	Rewards         []ChallengeBundleReward    `json:"rewards"`
	Objectives      []ChallengeBundleObjective `json:"objectives"`
	Options         ChallengeBundleOptions     `json:"options"`
}

type ChallengeBundleReward struct {
	TemplateID string `json:"templateId"`
	Quantity   int    `json:"quantity"`
}

type ChallengeBundleObjective struct {
	BackendName string `json:"backendName"`
	Count       int    `json:"count"`
	Stage       int    `json:"stage,omitempty"`
}

type ChallengeBundleOptions struct {
	IsBattlePass                  bool `json:"isBattlePass"`
	IsOvertime                    bool `json:"isOvertime"`
	GrantWithPass                 bool `json:"grantWithPass"`
	ProgressOnBattlePassPurchased bool `json:"progressOnBattlePassPurchased"`
	AthenaSeasonProgress          bool `json:"athenaSeasonProgress"`
	BattlePassProgress            bool `json:"battlePassProgress"`
	GainAthenaSeasonXP            bool `json:"gainAthenaSeasonXP"`
}

type BundleCompletionReward struct {
	TemplateID string `json:"templateId"`
	Quantity   int    `json:"quantity"`
}

type ChallengeBundleSchedule struct {
	TemplateID  string `json:"templateId"`
	QuestBundle string `json:"questBundle"`
}
