package data

import "github.com/mylxsw/eloquent/migrate"

func Migrate20240125DML(m *migrate.Manager) {
	m.Schema("20240125-dml").Raw("prompt_example", func() []string {
		return []string{
			"INSERT INTO prompt_example (title, content, models, tags, created_at, updated_at) VALUES ('', '海底世界', null, '[\"artistic-wordart\"]', DEFAULT, DEFAULT)",
			"INSERT INTO prompt_example (title, content, models, tags, created_at, updated_at) VALUES ('', '乐高积木', null, '[\"artistic-wordart\"]', DEFAULT, DEFAULT)",
			"INSERT INTO prompt_example (title, content, models, tags, created_at, updated_at) VALUES ('', '花卉盛开', null, '[\"artistic-wordart\"]', DEFAULT, DEFAULT)",
			"INSERT INTO prompt_example (title, content, models, tags, created_at, updated_at) VALUES ('', '雪域高原', null, '[\"artistic-wordart\"]', DEFAULT, DEFAULT)",
		}
	})
}
