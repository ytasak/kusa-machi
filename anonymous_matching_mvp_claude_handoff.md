# 匿名マッチング MVP — 実装引き継ぎ仕様書

## 0. ゴール

`kusa` SNS の iframe 内で動き、かつ自分の URL から直接開いても使える、軽量な
匿名マッチングシミュレーションゲームを作る。

中心となるコンセプト:

- 毎日、各参加者にランダム生成された架空の Persona が1つ配られる。
- Persona の「ステータス」属性はその日のあいだ固定される。
- 参加者が編集できるのは、軽い自己表現の項目だけ。
- 参加者は他人の Persona を閲覧し、1日分の限られた Like を使う。
- 相互 Like で Match が成立する。
- ゲームデータはすべて一時的で、毎日 00:00 JST にリセットされる。
- 履歴・ランキング・チャット・永続プロフィールは存在しない。

主な体験:
1. `マッチング市場のシミュレーション`
2. 入口の体験としての日替わり Persona ガチャ
3. ロールプレイは任意で、ユーザーに委ねる

これは個人の趣味の MVP。拡張性や作り込みよりも、単純で明示的な実装を優先する。

---

# 1. 技術スタック

## バックエンド
- Go
- HTTP ルータ: `chi`
- PostgreSQL
- SQL アクセス: `sqlc`
- マイグレーション: `golang-migrate`

## フロントエンド
- Svelte + Vite
- CSS Modules

## デプロイ
- 1コンテナ構成でよい
- フロントエンドと API は同一 Origin で配信する
- 次のどちらでも動くこと:
  - kusa の iframe 内
  - URL を直接開いた場合

---

# 2. プロダクトの原則

## 2.1 日次の一時性

ゲームデータはすべて1つの暦日にスコープされる。

00:00 JST に:
- その日の Persona が失効する
- Like が失効する
- Pass が失効する
- Match が失効する
- 参加者が入力した 名前 / 趣味 / ひとこと が失効する
- 前日のゲームデータは即座にアクセス不能になる
- 古いデータは非同期に物理削除される

存在しないもの:
- 履歴
- 累積統計
- 連続記録
- ランキング
- 過去の Persona
- 過去の Match
- 過去の Like 数

## 2.2 アカウントシステムなし

kusa は iframe 内のアプリにユーザー identity を提供しない。

ブラウザは匿名 Cookie で識別する。

Cookie を消せば新しい参加者 identity を得られてしまうが、MVP ではそれを許容する。
再抽選対策は不要。

---

# 3. ユーザー identity のモデル

3つの概念を明確に分離し続けること。

## `cookie_token`
ブラウザの識別子。

- 不透明なランダム ID
- UUID v4 かそれ相当
- Cookie にのみ保存する
- 有効期限は約30日
- ゲームの状態は Cookie に保存しない

Cookie の属性:
- `HttpOnly`
- `Secure`
- `SameSite=None`
- `Path=/`
- `Domain` は明示しない

## `Participant`
1ゲーム日ぶんの技術的な参加者。

関係:

```text
cookie_token -> Participant -> Persona
```

`cookie_token + game_date` につき Participant は1つ。

Participant 自体はゲーム内には現れない。

## `Persona`
マッチング市場で使われる架空の人物。

ゲーム内の操作はすべて `persona_id` を使い、Participant や Cookie の identity は使わない。

Like / Pass / Match の主体は Persona。

---

# 4. 1日のライフサイクル

タイムゾーン: **Asia/Tokyo**

## その日の初回アクセス

1. 匿名 Cookie を読むか発行する。
2. その日の Participant の存在を保証する。
3. その日の Persona が存在しない場合:
   - 「新しい人生を始める」を表示する
4. 押されたら:
   - サーバが Persona を一度だけ生成する
   - 即座に保存する
   - Persona は即座に公開され、市場に出る
5. 1〜2秒の短い生成アニメーションを見せる。
6. 生成された属性を一度にすべて開示する。

Persona 生成 API は冪等であること:
- その日の Persona が無い → 生成する
- すでに生成済み → 既存の Persona を返す
- 同じ日のうちに振り直しは絶対にしない

## 1日の終わり

常時、次のようなカウントダウンを表示する:

```text
今日の人生 残り 03:42:18
```

23:55 に、アプリが開かれていれば、アプリ内モーダルを表示する:

```text
今日の人生はあと5分です
```

00:00 に:
- その日は即座に無効になる
- 開いているクライアントには次を表示する:
  - `今日の人生が終了しました`
- 翌日の Persona を作るには、ユーザーが `新しい人生を始める` を押す必要がある

正しさを 00:00 の削除ジョブに依存させないこと。
すべてのクエリが `game_date = 当日(JST)` でデータをスコープすること。

---

# 5. Persona のモデル

## 5.1 システム生成の不変属性（A）

1日に一度だけ生成され、以後不変:

- 年齢
- 性別
- 身長
- 学歴
- 職業
- 年収

## 5.2 ユーザーが編集できる属性（B）

その日のあいだ何度でも編集できる:

- 名前
- 趣味
- ひとこと
- プロフィール写真 （2026-08-16 追加。§23 の Non-Goal から外した）

制限:
- 名前: 最大20文字
- 趣味: 最大30文字
- ひとこと: 最大60文字

ルール:
- 単一行のみ
- 改行は不可
- 前後の空白は除去する
- 空白のみの場合は未設定（null）にする
- プレーンテキストのみ
- Markdown は解釈しない
- HTML として描画しない
- 出力時はエスケープする
- 明示的な URL（`http://`、`https://`）は禁止
- 電話番号・メールアドレス・SNS ID など、外部連絡先のあらゆる表記を検出しようとはしない
- MVP では NG ワードフィルタを持たない

名前 / 趣味 / ひとこと が未設定の場合、その項目は Persona カードから丸ごと省く。

## 5.3 プロフィール写真

任意。未設定のときはシンプルな既定のシルエットを表示する。

クライアント側:
- アップロード前に中央で正方形に切り抜き、長辺1024px以下に縮小する
- これはアップロードを小さくするためだけのもので、セキュリティ制御ではない

サーバ側（信用してよい唯一の場所）:
- JPEG と PNG のみ受理し、それ以外は拒否する
- 8MB を超えるボディと4000万画素を超える画像を拒否する。画素数チェックは
  画像をデコードする前にヘッダに対して行う
- 必ずデコードして JPEG として再エンコードする。これにより EXIF が消える
  （匿名アプリが誰かの GPS 座標を公開してはならない）とともに、画素に紛れ込ませた
  ものも捨てられる
- 長辺1024pxで保存する

保存と寿命:
- `PHOTO_DIR` 配下のファイルとして保存し、`game_date` ごとにディレクトリを分ける
- 日次クリーンアップが今日より古いディレクトリを削除するため、写真もその日の
  他のデータと一緒に失効する
- 配信時は `Content-Type: image/jpeg` と `X-Content-Type-Options: nosniff` を付け、
  当日の Persona のものだけを配信する
- 参加者はいつでも自分の写真を削除できる

この MVP で受け入れている既知の制約: 通報の導線もモデレーションも無いため、
アップロードされた写真は本人が削除するか日次リセットが来るまで残る。

---

# 6. Persona カードの表示順

0. プロフィール写真（未設定なら既定のシルエット）
1. 名前（設定されている場合のみ）
2. 年齢 + 性別
3. 身長
4. 職業
5. 年収
6. 学歴
7. 趣味（設定されている場合のみ）
8. ひとこと（設定されている場合のみ）

公開 Persona カードに表示してはならないもの:
- Like 数
- Match 数
- 人気ランク
- レア度
- 総合スコア
- 露出回数
- Participant ID
- Cookie 識別子

Persona カードの UI コンポーネントは次の画面で共通のものを使う:
- 探す
- Likeされた
- 送信済みLike
- Match

画面ごとに変えてよいのは、アクションとバッジだけ。

---

# 7. Persona の生成

生成順:

```text
年齢
 -> 性別
 -> 身長
 -> 学歴
 -> 職業
 -> 年収
```

目指すのは**最低限の妥当性の制約**であって、現実的な人口統計のシミュレーションではない。

## 7.1 年齢

範囲: 20〜50

重み付きの年齢帯:

| 範囲 | 重み |
|---|---:|
| 20–24 | 20% |
| 25–29 | 25% |
| 30–34 | 20% |
| 35–39 | 15% |
| 40–44 | 10% |
| 45–50 | 10% |

手順:
1. 重みに従って帯を選ぶ
2. その帯の中から一様に年齢を選ぶ

## 7.2 性別

- male: 50%
- female: 50%

MVP では性別によるフィルタリングを行わない。

## 7.3 身長

一様乱数:

```text
140–200 cm
```

1cm 刻み。

性別による補正はしない。

## 7.4 学歴

| 学歴 | 重み |
|---|---:|
| 中卒 | 5% |
| 高卒 | 20% |
| 専門卒 | 15% |
| 短大卒 | 10% |
| 大卒 | 35% |
| 大学院卒 | 10% |
| ホイ卒 | 5% |

年齢による制約:
- 大卒: 22歳以上
- 大学院卒: 24歳以上
- その他: 20歳以上

不適格な候補は除外し、重みを再正規化する。

### 特例: ホイ卒
`ホイ卒` は意図的な kusa の内輪ネタであり、レアなミーム学歴。

学歴が `ホイ卒` の場合、職業の制約はすべて無視する。

## 7.5 職業

| 職業 | 重み |
|---|---:|
| 公務員 | 7% |
| 医師 | 2% |
| 看護師 | 6% |
| 教員 | 6% |
| ITエンジニア | 10% |
| 営業 | 10% |
| 事務 | 10% |
| 販売・接客 | 10% |
| 飲食 | 8% |
| 建設 | 8% |
| クリエイター | 6% |
| 自営業 | 6% |
| 経営者 | 2% |
| フリーター | 5% |
| 無職 | 4% |

制約:
- 医師:
  - 24歳以上
  - 学歴が 大卒 か 大学院卒
- 教員:
  - 22歳以上
  - 学歴が 大卒 か 大学院卒
- 経営者:
  - 25歳以上
- 看護師:
  - 追加の制約なし
- その他:
  - 追加の制約なし
- 学歴が `ホイ卒` の場合:
  - 職業の制約をすべて無視する

不適格な候補は除外し、重みを再正規化する。

## 7.6 年収

保存単位: **万円**

値はすべて10万円刻みであること。

| 職業 | レンジ（万円） |
|---|---:|
| 公務員 | 300–750 |
| 医師 | 700–1800 |
| 看護師 | 300–750 |
| 教員 | 300–750 |
| ITエンジニア | 300–900 |
| 営業 | 300–900 |
| 事務 | 250–550 |
| 販売・接客 | 250–550 |
| 飲食 | 250–550 |
| 建設 | 300–900 |
| クリエイター | 200–1200 |
| 自営業 | 200–1200 |
| 経営者 | 300–3000 |
| フリーター | 100–300 |
| 無職 | 0–100 |

年齢による補正は意図的に弱くする:
- 20代: やや低め〜中程度に寄せる
- 30代: ほぼ平坦
- 40〜50代: やや中程度〜高めに寄せる

極端な組み合わせも起こりうる状態を保つこと。

ここは作り込まない。単純に歪ませた乱数で十分。

レア度や「良い人生／悪い人生」のスコアは一切計算しない。

---

# 8. マッチングのルール

## 1日の Like 予算

各 Persona がその日に持つ Like はちょうど:

```text
10 Like / 日
```

ルール:
- Pass は無制限
- Like は即座に消費される
- Like は取り消せない
- 使わなかった Like は繰り越されない
- Like 予算は新しい日にリセットされる
- Like の上限はサーバ側のトランザクションで強制する
- 同じリクエストが重複しても Like を二重に消費しない
- 同じ相手に Like できるのは1日1回まで
- 自分への Like は禁止

Likeされた相手へのお返しも、同じ10 Likeの予算を共有する。

返信用の別枠は存在しない。

## Match

相互 Like で Match が1つ成立する。

Match:
- 順序を持たない
- Persona のペアにつき1つ
- 冪等であること
- 都度計算するのではなく明示的に保存する

相互 Like が成立したとき:
- Like API は `matched = true` を返す
- フロントエンドは Match アニメーションを表示する
- Match が Match 一覧に現れる

MVP にチャットや DM は無い。

---

# 9. Pass のルール

Pass は初回で永久除外にはならない。

一度 Pass した Persona がまた表示されることがある。

ルール:
- Persona ペアの方向ごとに `pass_count` を保持する
- 1回目の Pass → あとでまた出てよい
- 2回目の Pass → あとでまた出てよい
- 3回目の Pass → その日はもう出さない
- 自分への Pass は禁止
- 相手が Like 済み / Match 済みになったら、もう Pass の操作はできない

クールダウン:
- Pass した直後の Persona は、次に表示・評価される5枚のあいだは再表示しない
- MVP ではこの5枚のクールダウンをフロントエンドのセッション状態で保持してよい
- ページを再読み込みするとクールダウンが失われるが、それは許容する
- `pass_count = 3` はサーバ側で保持し、その日のあいだ永続する

---

# 10. 探す / 市場での露出

探す画面の UI:
- 一度に1枚の Persona カード
- Like ボタン
- Pass ボタン
- MVP にスワイプ操作は無い
- 確認ダイアログは無い
- Pass は素早く次へ遷移する
- Like には短い `LIKE` のフィードバックアニメーションを付ける
- Match したときは直接 Match アニメーションへ遷移する

常に表示するもの:
- 残り Like `N / 10`
- Likeされた件数の小さなバッジ

探す画面に Match 数は表示しない。

## 候補の選び方

`GET /api/discover` は一度に最大5件の Persona を返す。

選択の条件:
- 自分ではない
- 当日の `game_date`
- Persona が存在し有効である
- リクエスト元がまだ Like していない
- まだ Match していない
- `pass_count < 3`
- フロントエンドから除外 ID が渡された場合はそれを尊重する
- 1回のレスポンス内に同じ Persona を含めない
- 前回のバッチと重複するのは構わない

優先度:
1. `exposure_count` が少ない順
2. 同程度の露出の中ではランダム順

## 露出のカウント

探す API がバッチを返した時点では露出を加算しない。

次のいずれかをユーザーが確定したときにのみ:

```text
exposure_count += 1
```

- Like
- または Pass

つまり露出は「実際に評価されたプロフィール」を表す。

フロントエンドは現在のバッチが尽きかけたら自動的に次のバッチを先読みする。

バッチの切れ目がユーザーから見えてはならない。

---

# 11. Likeされた

画面:
- 現在の Persona に Like した Persona を一覧表示する
- 新しい順
- 公開 Persona カードをそのまま表示する
- Like の時刻は表示しない
- 送信者の Like 予算の情報は表示しない

ユーザーは Like を返せる。

Like を返す操作:
- 同じ1日10 Likeの予算を消費する
- 相互 Like になれば Match が成立する

マイページ:
- Likeされた件数を表示する
- 未読の Like があれば目立つ形で表示する:
  - `新しいLikeがあります`
- Likeされた画面を開いたらバッジを消す

リアルタイム通信は不要。
状態は画面遷移やリクエストのタイミングで更新される。

---

# 12. 送信済みLike

画面:
- 現在の Persona が Like した Persona を一覧表示する
- 新しい順
- Like は取り消せない
- Match した相手も一覧に残る
- Match した相手には `MATCH` バッジを付ける

この画面はその日の Like 配分の履歴として機能する。

データはすべて 00:00 に消える。

---

# 13. Match

画面:
- Match した相手の Persona を一覧表示する
- 新しい順
- 相手の Persona カードのみ表示する
- 自分の Persona は繰り返し表示しない
- 読み取り専用

マイページ:
- その日の Match 数を表示する
- 未読の Match があれば目立つ形で表示する:
  - `新しいMatchがあります！`
- Match 画面を開いたらバッジを消す

Match 成立時のアニメーション:
- 自分の Persona と相手の Persona を並べて表示する
- 短く `MATCH!` を出す
- コピーには次を含めてよい:
  - `今日の人生でマッチしました`

チャットや DM は無い。

---

# 14. ナビゲーションとマイページ

> **改訂。** この節はもともと「ホームがナビゲーションの起点。常時表示の
> ボトムタブは持たない」だった。2026-08-16 にこれを取り消した。ここで唯一
> 想定している端末であるスマートフォンでは、常時表示のタブバーのほうが
> 片手で届きやすいため。ホーム画面はマイページに置き換わり、カウントダウンは
> 常時表示のヘッダーへ移してすべての画面で見えるようにする。

## 常時表示の要素

その日の Persona が存在するようになると、すべての画面でヘッダーを表示する:

1. 日次カウントダウン
2. 残り Like `N / 10`

その日の Persona が存在するようになると、すべての画面でボトムタブバーを表示する。
タブはナビゲーションの優先順に:

1. 探す
2. Likeされた
3. Match
4. マイページ

未読の Like と未読の Match は、それぞれのタブにドットを出す。

その日の Persona が存在しないあいだは「新しい人生を始める」画面がアプリ全体を
占有する。ヘッダーもタブバーも出さない。

## マイページ

主要な情報:
1. その日の自分の Persona カード
2. 残り Like `N / 10`
3. Likeされた件数
4. その日の Match 数

マイページから遷移し、マイページへ戻る導線を持つ画面:

- 送信済みLike
- プロフィール編集

---

# 15. プロフィール編集画面

上部:
- 不変の A属性を読み取り専用で表示する

その下で編集できるもの:
- 名前
- 趣味
- ひとこと
- プロフィール写真

4つとも任意。
変更は公開 Persona プロフィールに即座に反映される。

「公開する」という状態は無い。
B属性がすべて空でも、Persona は生成された時点で公開される。

---

# 16. 主要な画面

MVP の主要画面はちょうど次のとおり:

1. 探す （タブ）
2. Likeされた （タブ）
3. Match （タブ）
4. マイページ （タブ。元のホーム画面を置き換える — §14 を参照）
5. 送信済みLike （マイページから遷移）
6. プロフィール編集 （マイページから遷移）

補助的な UI / モーダルの状態:
- 新しい人生を始める
- Persona 生成アニメーション
- Match アニメーション
- 23:55 の5分前警告
- 00:00 の終了

---

# 17. フロントエンドの状態の扱い

探す画面は5件ずつバッチで取得する。

同一ページセッションのあいだ保持するもの:
- 現在の探すカード
- 取得済みのバッチ
- バッチ内の位置
- ローカルな5枚ぶんの Pass クールダウン除外リスト

探す → Likeされた → 探す と移動した場合:
- 直前のカード / バッチから再開する

ページを完全に再読み込みした場合やブラウザを再起動した場合:
- 探す画面のメモリ上の状態は失われてよい
- 取り直す
- まだ操作していない Persona が再登場してよい
- これは許容する

次のものについてはサーバの状態が正:
- Like
- Pass 回数
- Match
- Persona
- Like 予算

---

# 18. データベーススキーマ

主キーは UUID を使う。

## participants

```sql
CREATE TABLE participants (
    id UUID PRIMARY KEY,
    cookie_token UUID NOT NULL,
    game_date DATE NOT NULL,
    csrf_token TEXT NOT NULL,
    likes_last_seen_at TIMESTAMPTZ NULL,
    matches_last_seen_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (cookie_token, game_date)
);
```

## personas

```sql
CREATE TABLE personas (
    id UUID PRIMARY KEY,
    participant_id UUID NOT NULL UNIQUE
        REFERENCES participants(id)
        ON DELETE CASCADE,

    age SMALLINT NOT NULL,
    gender TEXT NOT NULL,
    height_cm SMALLINT NOT NULL,
    education TEXT NOT NULL,
    occupation TEXT NOT NULL,
    annual_income INTEGER NOT NULL,

    name VARCHAR(20) NULL,
    hobby VARCHAR(30) NULL,
    bio VARCHAR(60) NULL,

    exposure_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

プロフィール写真の追加にあわせて `photo_updated_at TIMESTAMPTZ NULL` を後から
追加した（§5.3 を参照）。写真の実体は DB ではなくファイルとして保存する。

推奨する CHECK 制約:
- age は 20 以上 50 以下
- height_cm は 140 以上 200 以下
- annual_income >= 0
- exposure_count >= 0

## likes

```sql
CREATE TABLE likes (
    id UUID PRIMARY KEY,
    from_persona_id UUID NOT NULL
        REFERENCES personas(id)
        ON DELETE CASCADE,
    to_persona_id UUID NOT NULL
        REFERENCES personas(id)
        ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (from_persona_id, to_persona_id),

    CHECK (from_persona_id <> to_persona_id)
);
```

## passes

```sql
CREATE TABLE passes (
    id UUID PRIMARY KEY,
    from_persona_id UUID NOT NULL
        REFERENCES personas(id)
        ON DELETE CASCADE,
    to_persona_id UUID NOT NULL
        REFERENCES personas(id)
        ON DELETE CASCADE,

    pass_count SMALLINT NOT NULL DEFAULT 1,
    last_passed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (from_persona_id, to_persona_id),

    CHECK (from_persona_id <> to_persona_id),
    CHECK (pass_count BETWEEN 1 AND 3)
);
```

## matches

```sql
CREATE TABLE matches (
    id UUID PRIMARY KEY,

    persona_low_id UUID NOT NULL
        REFERENCES personas(id)
        ON DELETE CASCADE,

    persona_high_id UUID NOT NULL
        REFERENCES personas(id)
        ON DELETE CASCADE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (persona_low_id, persona_high_id),

    CHECK (persona_low_id <> persona_high_id)
);
```

アプリケーション側が INSERT 前に Persona のペアを正規化すること。

---

# 19. 日次の物理削除

正しさは削除のタイミングではなく `game_date` で担保する。

削除ジョブ:
- 定期的に `participants WHERE game_date < 当日(JST)` を削除する
- `ON DELETE CASCADE` により次も削除される:
  - personas
  - likes
  - passes
  - matches
- プロフィール写真は `ON DELETE CASCADE` の対象外なので、`game_date` ごとの
  ディレクトリを削除して掃除する（§5.3 を参照）

ジョブは冪等でリトライ可能であること。

履歴の永続保管はしない。

---

# 20. CSRF / Web セキュリティ

identity が Cookie ベースで、かつ埋め込まれうるアプリなので:

## CSRF

Participant は日次の CSRF トークンを1つ保持する。

生成:
- 暗号論的に安全な乱数
- 32バイト程度
- Base64URL 相当の不透明なトークン

ライフサイクル:
- その日の Participant に対して生成する
- 有効なのはそのゲーム日のあいだだけ
- 初期化 / ホームの取得時に返す
- フロントエンドはメモリ上に保持する
- 更新系リクエストは次のようなヘッダで送る:

```text
X-CSRF-Token: ...
```

CSRF トークンを必須とするもの:
- Persona 生成
- プロフィール更新
- プロフィール写真のアップロードと削除
- Like
- Pass
- 今後追加されるあらゆる更新系エンドポイント

GET のエンドポイントはゲーム状態を変更しないこと。ただし後述の「最終閲覧時刻」の
更新だけは明示的に許容する。

## XSS

B属性はすべて:
- プレーンテキスト
- 描画時にエスケープする
- 生の HTML を挿入しない
- Markdown を解釈しない

## サーバ側の検証

次についてフロントエンドを信用しないこと:
- Like 数
- Like 予算
- Persona の属性
- Match の生成
- Pass 回数
- 文字数制限
- 改行の禁止
- 明示的な URL の禁止
- アップロードされた画像の形式・寸法・容量

---

# 21. API

ベースパス:

```text
/api
```

成功時のペイロードに共通のラッパーは不要。

エラーは次の形式:

```json
{
  "error": {
    "code": "LikeLimitExceeded",
    "message": "今日のLikeを使い切りました"
  }
}
```

フロントエンドは `error.code` で分岐すること。

## ドメインのエラーコード

- `PersonaNotGenerated`
- `LikeLimitExceeded`
- `AlreadyLiked`
- `TargetPersonaUnavailable`
- `PassLimitReached`
- `SelfActionNotAllowed`
- `DayExpired`
- `InvalidProfileInput`

HTTP との対応:

| エラー | HTTP |
|---|---:|
| InvalidProfileInput | 400 |
| PersonaNotGenerated | 404 |
| TargetPersonaUnavailable | 404 |
| AlreadyLiked | 409 |
| PassLimitReached | 409 |
| DayExpired | 409 |
| LikeLimitExceeded | 422 |
| SelfActionNotAllowed | 422 |

---

## GET `/api/home`

責務:
- その日の Participant の存在を保証する
- アプリの現在の状態を返す

例:

```json
{
  "server_time": "2026-08-16T21:00:00+09:00",
  "game_date": "2026-08-16",
  "persona_generated": true,
  "persona": {},
  "remaining_likes": 7,
  "received_like_count": 4,
  "match_count": 2,
  "has_unseen_likes": true,
  "has_unseen_matches": false,
  "csrf_token": "..."
}
```

Persona が未生成の場合:
- `persona_generated=false`
- `persona` は null でよい

---

## POST `/api/persona`

CSRF 必須。

冪等:
- その日の Persona が無い → 生成して保存する
- ある → 既存の Persona を返す

振り直しは絶対にしない。

---

## GET `/api/persona/me`

自分の Persona を A属性 + B属性 込みで返す。

---

## PATCH `/api/persona/profile`

CSRF 必須。

リクエスト:

```json
{
  "name": "...",
  "hobby": "...",
  "bio": "..."
}
```

受け付けるのは B属性のみ。

A属性を黙って受け入れないこと。

---

## POST `/api/persona/photo`

CSRF 必須。

ボディは生の画像。サーバが必ずデコードして再エンコードする（§5.3 を参照）。
更新後の Persona カードを返す。

---

## DELETE `/api/persona/photo`

CSRF 必須。

プロフィール写真を削除し、更新後の Persona カードを返す。

---

## GET `/api/personas/{persona_id}/photo`

当日の Persona の写真を配信する。写真が無い場合や当日の Persona でない場合は
`TargetPersonaUnavailable` を返す。

---

## GET `/api/discover`

公開 Persona カードを最大5件返す。

任意のクエリ:

```text
?exclude=id1,id2,id3
```

フロントエンドは現在のクールダウン除外 ID を送ってよい。

このレスポンスで露出を加算しないこと。

---

## POST `/api/likes`

CSRF 必須。

リクエスト:

```json
{
  "persona_id": "..."
}
```

トランザクション内で:
1. 当日 / 自分の Persona を検証する
2. 対象を検証する
3. 自分自身なら拒否する
4. 重複なら拒否する
5. 送信済み Like 数 < 10 を強制する
6. Like を INSERT する
7. 対象の exposure_count を加算する
8. 逆方向の Like を確認する
9. 逆方向が存在すれば、正規化した Match を冪等に作成する

レスポンス例:

```json
{
  "remaining_likes": 6,
  "matched": true,
  "match_id": "...",
  "target_persona": {}
}
```

Match していない場合:
- `match_id` / `target_persona` は省略または null でよい

---

## POST `/api/passes`

CSRF 必須。

リクエスト:

```json
{
  "persona_id": "..."
}
```

トランザクション内で:
1. 当日 / 対象を検証する
2. 自分自身なら拒否する
3. 対象の状態が不正なら拒否する
4. Pass を INSERT するか加算する
5. 3 で打ち止めにする
6. 対象の exposure_count を加算する

レスポンス:

```json
{
  "pass_count": 2,
  "excluded_for_today": false
}
```

---

## GET `/api/likes/received`

現在の Persona に Like した Persona を返す。
新しい順。

この画面を開くと Like を既読にするため次を更新する:

```text
participants.likes_last_seen_at
```

MVP にページングは無い。

---

## GET `/api/likes/sent`

現在の Persona が Like した Persona を返す。
新しい順。

各要素に次を追加で含める:

```json
{
  "matched": true
}
```

ページングは無い。

---

## GET `/api/matches`

Match した相手の Persona を返す。
新しい順。

この画面を開くと次を更新する:

```text
participants.matches_last_seen_at
```

ページングは無い。

---

# 22. トランザクションと並行性の重要な要件

## Persona の生成
競合に対して安全であること。

DB の一意制約:
- `(cookie_token, game_date)`
- `personas(participant_id)`

同時にリクエストが来た場合:
- 同じ Persona を返すこと

## Like 予算
サーバ側のトランザクションで強制すること。

2つのタブから操作しても10件を超えて Like できてはならない。

重複リトライ:
- Like を二重に消費してはならない

## Match
冪等であること。

ID を正規化する:

```text
low_id = min(personaA, personaB)
high_id = max(personaA, personaB)
```

DB の一意制約が Match の重複を防ぐ。

---

# 23. MVP で作らないもの

コアの体験を成立させるために必要でない限り、次は実装しない:

- チャット
- DM
- ブロック
- 通報
- モデレーション画面
- プッシュ通知
- ブラウザ通知
- WebSocket
- SSE
- スワイプ操作
- 履歴
- 累積統計
- ランキング
- Like 率
- ユーザーに見せる露出統計
- レア度 / SSR / スコア
- 検索
- 性別フィルタ
- マッチング条件
- ページング
- ソーシャルログイン
- kusa 連携 API
- 再抽選対策 / Cookie 削除対策
- 高度な不正対策
- NG ワード辞書
- 電話番号 / メール / SNS ID の検出
- 長期プロフィール
- 顔の魅力度スコア
- 「親の資産」のような追加ステータス

---

# 24. 完成の定義

次の一連の流れが通ればこの MVP は完成:

```text
アプリを開く
  ↓
匿名 Cookie を発行 / 読み取り
  ↓
その日の Participant を作成
  ↓
「新しい人生を始める」
  ↓
サーバがランダムな Persona を1つ生成
  ↓
Persona が即座に市場へ参加
  ↓
任意で 名前 / 趣味 / ひとこと / 写真 を編集
  ↓
探す画面を開く
  ↓
候補を5件取得
  ↓
1枚ずつ Persona を表示
  ↓
Like / Pass
  ↓
10 Like の予算が強制される
  ↓
Likeされた を閲覧できる
  ↓
Like を返せる
  ↓
相互 Like で Match が成立
  ↓
Match アニメーションが出る
  ↓
送信済みLike と Match を確認できる
  ↓
マイページにカウンタ、ヘッダーに日次カウントダウン
  ↓
23:55 の警告
  ↓
00:00 に前日の状態が無効になる
  ↓
「今日の人生が終了しました」
  ↓
翌日の「新しい人生を始める」
```

これが安定して動くなら、機能追加をやめて実ユーザーで試すこと。

---

# 25. プロジェクト構成の案

名前は調整してよいが、責務は明示的に保つこと。

```text
.
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── participant/
│   ├── persona/
│   ├── matching/
│   ├── photo/
│   ├── http/
│   │   ├── handler/
│   │   ├── middleware/
│   │   └── response/
│   ├── db/
│   │   ├── query/
│   │   └── sqlc/
│   └── clock/
├── migrations/
├── web/
│   ├── src/
│   │   ├── lib/
│   │   ├── components/
│   │   ├── routes/
│   │   └── stores/
│   └── vite.config.*
├── sqlc.yaml
├── go.mod
└── README.md
```

実際の構成は README を参照。

アーキテクチャ上の潔癖さのためだけに、repository / service / usecase の層を
作り込まないこと。次のものが明確でテスト可能である程度の分離にとどめる:
- HTTP
- ドメインのルール
- SQL
- フロントエンド

---

# 26. 実装の推奨順

1. Go + chi + PostgreSQL + sqlc + マイグレーションの足場
2. JST のゲーム日付クロックの抽象化
3. Cookie による participant identity
4. Participant の ensure ロジック
5. マイグレーションとスキーマの作成
6. Persona 生成器 + ユニットテスト
7. Persona 生成エンドポイント
8. ホームのエンドポイント
9. プロフィール編集の検証
10. 探すクエリ
11. Pass のトランザクション
12. Like のトランザクション + 10件上限
13. Match の生成
14. 一覧 API
15. CSRF
16. Svelte のシェル / マイページ
17. Persona カード
18. 新しい人生を始めるフロー
19. 探すフロー + 5枚の先読み
20. Likeされた / 送信済みLike / Match の各画面
21. Match アニメーション
22. 日次カウントダウン + 23:55 + 00:00 の遷移
23. 物理削除ジョブ
24. iframe 内の動作確認
25. 直接 URL での動作確認
26. Persona 生成と10 Like予算まわりの並行性テスト

---

# 27. 重要なテスト

次を優先する。

## Persona 生成
- 年齢が常に 20〜50
- 身長が常に 140〜200
- 大卒が22歳未満にならない
- 大学院卒が24歳未満にならない
- ホイ卒を除き医師の制約が守られる
- ホイ卒を除き教員の制約が守られる
- ホイ卒を除き経営者の年齢制約が守られる
- 年収が常に職業のレンジ内
- 年収が常に10の倍数

## Like
- 自分に Like できない
- 同じ相手に重複して Like できない
- 10件の Like が成功する
- 11件目の Like が失敗する
- 同時に Like しても10件を超えない
- リトライで二重に消費しない
- 相互 Like で Match が1件だけできる

## Pass
- pass_count が 1 → 2 → 3 と増える
- 3 で除外される
- 3 を超えない
- 成功した操作1回につき exposure が1増える

## 日付境界
- 前日の Persona は当日に行動できない
- 前日の Like / Match が当日に現れない
- 同じ Cookie で翌日の Participant が作れる
- 翌日の Persona が生成できる
- 前日の Persona は再利用されない

## セキュリティ
- 更新系で不正な CSRF が拒否される
- プロフィールの PATCH で A属性を変更できない
- 改行が拒否される
- 明示的な URL が拒否される
- HTML がテキストとして扱われる

## プロフィール写真
- 画像でないアップロードが拒否される
- 展開爆弾がデコード前に拒否される
- 再エンコードで EXIF が消える
- 保存される画像が長辺1024px以下になる
- 前日ぶんの写真が日次クリーンアップで消える

---

# 28. 最後に

コアとなる1日のループを中心に MVP を実装すること。

指定が無い箇所では:
1. 最も単純な実装を選ぶ
2. 日次の一時性を守る
3. 10 Like という市場の制約を守る
4. Persona と User の分離を守る
5. 機能を足さない
6. 現在の MVP が正当化しないアーキテクチャを持ち込まない

実装を進める中でこの仕様に本当の矛盾が見つかった場合は、勝手に新しいプロダクト
ルールを作らず、その矛盾を明示して止まること。
