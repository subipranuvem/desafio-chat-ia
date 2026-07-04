package repository

import (
	"context"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/database"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
)

type postgresMessageRepository struct {
	db database.PostgresDB
}

func NewPostgresMessageRepository(db database.PostgresDB) MessageRepository {
	return &postgresMessageRepository{db: db}
}

func (r *postgresMessageRepository) SaveMessages(ctx context.Context, sessionID string, messages []model.Message) error {
	pool := r.db.Pool()
	for _, msg := range messages {
		_, err := pool.Exec(ctx,
			`INSERT INTO messages (session_id, role, content, input_token, output_token, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			sessionID, msg.Role, msg.Content, msg.InputToken, msg.OutputToken, msg.CreatedAt,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *postgresMessageRepository) GetMessages(ctx context.Context, sessionID string, limit, offset int) ([]model.Message, int64, error) {
	rows, err := r.db.Pool().Query(ctx,
		`SELECT id, role, content, input_token, output_token, created_at, COUNT(*) OVER() AS total
		 FROM messages
		 WHERE session_id = $1
		 ORDER BY created_at ASC
		 LIMIT $2 OFFSET $3`,
		sessionID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	messages := make([]model.Message, 0, limit)
	var total int64
	for rows.Next() {
		var msg model.Message
		if err := rows.Scan(&msg.ID, &msg.Role, &msg.Content, &msg.InputToken, &msg.OutputToken, &msg.CreatedAt, &total); err != nil {
			return nil, 0, err
		}
		messages = append(messages, msg)
	}
	return messages, total, rows.Err()
}

func (r *postgresMessageRepository) GetRecentMessages(ctx context.Context, sessionID string, limit int) ([]model.Message, error) {
	rows, err := r.db.Pool().Query(ctx,
		`SELECT id, role, content, input_token, output_token, created_at
		 FROM (
		     SELECT id, role, content, input_token, output_token, created_at
		     FROM messages
		     WHERE session_id = $1
		     ORDER BY created_at DESC
		     LIMIT $2
		 ) sub
		 ORDER BY created_at ASC`,
		sessionID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]model.Message, 0, limit)
	for rows.Next() {
		var msg model.Message
		if err := rows.Scan(&msg.ID, &msg.Role, &msg.Content, &msg.InputToken, &msg.OutputToken, &msg.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, rows.Err()
}
