package main

import (
	"context"
	// "encoding/json"
	"log"
	"time"

	"cloud.google.com/go/firestore"
)

type History struct {
  CreatedAt   time.Time `json:"created_at"`
  GptOutput   string    `json:"gpt_output"`
  UserInput   string    `json:"user_input"`
}

type ConversationHistory struct {
  EnrollmentID string    `json:"enrollment_id"`
  History      []History `json:"history"`
}

func saveHistory(ctx context.Context, client *firestore.Client, cv_h ConversationHistory) error {
  history_to_send := make([]map[string]string, len(cv_h.History))

  for i, h := range cv_h.History {
    values := map[string]string{"created_at": h.CreatedAt.String(), "gpt_output": h.GptOutput, "user_input": h.UserInput}

    // jsonValue, _ := json.Marshal(values)

    history_to_send[i] = values
  }

  _, err := client.Collection("conversation-history").Doc(cv_h.EnrollmentID).Set(ctx, map[string]interface{}{
          "history": history_to_send,
  }, firestore.MergeAll)

  if err != nil {
    // Handle any errors in an appropriate way, such as returning them.
    log.Printf("An error has occurred: %s", err)
  }

  return err
}
