package namedresolution

// Resolved holds one field per resolvable row. Each comment is the schema the
// field emits.
type Resolved struct {
	Tags     Tags     `json:"tags"`                              // -> {array, items: string}
	TagsPtr  *Tags    `json:"tagsPtr"`                           // -> {array, items: string}, no nullable
	TagsA    TagsA    `json:"tagsA"`                             // -> {array, items: string}
	Chain    Chain    `json:"chain"`                             // -> {array, items: string}
	Attrs    Attrs    `json:"attrs"`                             // -> {object, additionalProperties: string}
	Stamps   Stamps   `json:"stamps"`                            // -> {array, items: {string, date-time}}
	Ids      Ids      `json:"ids"`                               // -> {array, items: {integer, int64}}
	TagsV    Tags     `json:"tagsV" validate:"min=1,dive,max=5"` // -> {array, items: {string, maxLength 5}, minItems 1}
	AttrsV   Attrs    `json:"attrsV" validate:"min=1"`           // -> {object, additionalProperties: string, minProperties 1}
	Users    UserList `json:"users"`                             // -> {array, items: $ref User}
	PAddr    PAddr    `json:"pAddr"`                             // -> {object, allOf: [$ref Address], nullable}
	PC       PC       `json:"pc"`                                // -> {integer, int64, nullable}
	PCs      []PC     `json:"pcs"`                               // -> {array, items: {integer, int64}}
	FlagB    FlagB    `json:"flagB"`                             // -> {boolean}
	AnyD     AnyD     `json:"anyD"`                              // -> {}
	Shaper   Shaper   `json:"shaper"`                            // -> {}
	Pair     Pair     `json:"pair"`                              // -> {array, items: {integer, int64}}
	Key      Key      `json:"key"`                               // -> {array, items: {integer, int32, minimum 0}}
	Arr      Arr      `json:"arr"`                               // -> {array, items: {integer, int32, minimum 0}}
	Flags3   [3]Flag  `json:"flags3"`                            // -> {array, items: {integer, int32, minimum 0}}
	FlagPtrs []*Flag  `json:"flagPtrs"`                          // -> {array, items: {integer, int32, minimum 0}}

	Grid     [][]Cents          `json:"grid"`                     // -> {array, items: {array, items: {integer, int64}}}
	CentsMap map[string]Cents   `json:"centsMap"`                 // -> {object, additionalProperties: {integer, int64}}
	CentsSM  []map[string]Cents `json:"centsSM"`                  // -> {array, items: {object, additionalProperties: {integer, int64}}}
	UGrid    [][]User           `json:"uGrid"`                    // -> {array, items: {array, items: $ref User}}
	UGridPtr *[][]User          `json:"uGridPtr"`                 // -> {array, items: {array, items: $ref User}}
	USM      []map[string]User  `json:"usm"`                      // -> {array, items: {object, additionalProperties: $ref User}}
	UserMap  map[string]User    `json:"userMap" validate:"min=1"` // -> {object, additionalProperties: $ref User, minProperties 1}

	Blob      Blob            `json:"blob"`                                             // -> {string, byte}
	BlobPtr   *Blob           `json:"blobPtr"`                                          // -> {string, byte}, no nullable
	BlobA     BlobA           `json:"blobA"`                                            // -> {string, byte}
	B2        B2              `json:"b2"`                                               // -> {string, byte}
	Flags     Flags           `json:"flags"`                                            // -> {string, byte}
	Raw       Raw             `json:"raw"`                                              // -> {string, byte}
	RawOfRaw  RawOfRaw        `json:"rawOfRaw"`                                         // -> {string, byte}
	FlagSlice []Flag          `json:"flagSlice"`                                        // -> {string, byte}
	FlagGrid  [][]Flag        `json:"flagGrid"`                                         // -> {array, items: {string, byte}}
	FlagMap   map[string]Flag `json:"flagMap"`                                          // -> {object, additionalProperties: {integer, int32, minimum 0}}
	U8s       []U8            `json:"u8s"`                                              // -> {string, byte}
	FlagUs    []FlagU         `json:"flagUs"`                                           // -> {string, byte}
	FlagV     []Flag          `json:"flagV" validate:"min=1,dive,max=3" example:"AQI="` // -> {string, byte, example AQI=}

	Stamp   Stamp   `json:"stamp"`   // -> {string, date-time}
	IDA     IDA     `json:"ida"`     // -> {string, uuid}
	RawA    RawA    `json:"rawA"`    // -> {}
	Amount  Amount  `json:"amount"`  // -> {string}
	StatusD StatusD `json:"statusD"` // -> {integer, int64}
	Odd     Odd     `json:"odd"`     // -> {integer, int64}

	Members   Members   `json:"members"`   // -> {array, items: $ref Member}
	PMember   PMember   `json:"pMember"`   // -> {object, allOf: [$ref Member], nullable}
	MemberMap MemberMap `json:"memberMap"` // -> {object, additionalProperties: $ref Member}
	Holder    Holder    `json:"holder"`    // -> $ref Holder (item: $ref Item, items: {array, items: $ref Item})
}

// ResolvedQuery's params emit the hand-expanded param schema. The Marshaler
// types are bound by kind, so they are typed from their underlying type.
type ResolvedQuery struct {
	Tags    Tags   `query:"tags"`    // -> {array, items: string}
	TagsPtr *Tags  `query:"tagsPtr"` // -> {array, items: string}
	Ids     Ids    `query:"ids"`     // -> {array, items: {integer, int64}}
	FlagB   FlagB  `query:"flagB"`   // -> {boolean}
	Amount  Amount `query:"amount"`  // -> {string}
	TagsM   TagsM  `query:"tagsM"`   // -> {array, items: string}
	Status  Status `query:"status"`  // -> {integer, int64}
}
