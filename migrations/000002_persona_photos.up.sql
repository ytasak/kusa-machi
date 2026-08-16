-- プロフィール写真。
--
-- DBに持つのは「写真があるか、いつ更新されたか」の目印だけ。
-- 実体は PHOTO_DIR 配下のファイルで、game_date ごとのディレクトリに置く。
-- 日次クリーンアップがディレクトリごと消せば、写真も他のデータと一緒に消える。
ALTER TABLE personas ADD COLUMN photo_updated_at TIMESTAMPTZ NULL;
