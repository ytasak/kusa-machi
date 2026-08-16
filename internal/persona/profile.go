package persona

import (
	"strings"
	"unicode/utf8"

	"kusamachi/internal/apperr"
)

// ユーザーが編集できる「B属性」の文字数上限。バイトではなく文字数で数える。
const (
	MaxNameLen  = 20
	MaxHobbyLen = 30
	MaxBioLen   = 60
)

// Profile は検証・正規化済みの「B属性」を保持する。
// nil のフィールドは未設定であり、カードから項目ごと省く必要がある。
type Profile struct {
	Name  *string
	Hobby *string
	Bio   *string
}

// ProfileInput は PATCH の生のペイロード。フィールドが無い、または null の
// 場合は値をクリアする。編集画面は常に3つまとめて送るため、この更新は
// B属性の全置換として扱う。
type ProfileInput struct {
	Name  *string `json:"name"`
	Hobby *string `json:"hobby"`
	Bio   *string `json:"bio"`
}

// Validate は各フィールドを正規化・検証し、最初の違反で InvalidProfileInput を
// 返す。サーバはクライアント側の制限を一切信用しない。
func (in ProfileInput) Validate() (Profile, error) {
	name, err := normalizeField("name", in.Name, MaxNameLen)
	if err != nil {
		return Profile{}, err
	}
	hobby, err := normalizeField("hobby", in.Hobby, MaxHobbyLen)
	if err != nil {
		return Profile{}, err
	}
	bio, err := normalizeField("bio", in.Bio, MaxBioLen)
	if err != nil {
		return Profile{}, err
	}
	return Profile{Name: name, Hobby: hobby, Bio: bio}, nil
}

// normalizeField は前後の空白を除去し、空になった値を未設定に変え、
// 複数行・文字数超過・明示的な URL を拒否する。
//
// それ以外はそのまま保持する。値はプレーンテキストであり、描画時に
// フロントエンドがエスケープするため、HTML に見える入力もただの文字として表示される。
func normalizeField(field string, raw *string, maxLen int) (*string, error) {
	if raw == nil {
		return nil, nil
	}

	v := strings.TrimSpace(*raw)
	if v == "" {
		return nil, nil
	}

	if strings.ContainsAny(v, "\n\r") {
		return nil, apperr.New(apperr.CodeInvalidProfileInput, field+"に改行は使えません")
	}
	if utf8.RuneCountInString(v) > maxLen {
		return nil, apperr.New(apperr.CodeInvalidProfileInput, field+"が長すぎます")
	}
	if containsURL(v) {
		return nil, apperr.New(apperr.CodeInvalidProfileInput, field+"にURLは登録できません")
	}

	return &v, nil
}

// containsURL は仕様どおり、明示的な http/https の URL だけを拒否する。
// 電話番号・メールアドレス・SNS ID の検出は行わない。
func containsURL(v string) bool {
	lower := strings.ToLower(v)
	return strings.Contains(lower, "http://") || strings.Contains(lower, "https://")
}
