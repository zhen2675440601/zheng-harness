package main

import (
	"fmt"
	"zheng-harness/internal/store"
)

func main() {
	// 打开数据库
	userStore, err := store.NewSQLiteUserStore("agent.db")
	if err != nil {
		fmt.Printf("Error opening user store: %v\n", err)
		return
	}
	defer userStore.Close()

	sessionStore, err := store.NewSQLiteSessionStore("agent.db")
	if err != nil {
		fmt.Printf("Error opening session store: %v\n", err)
		return
	}
	defer sessionStore.Close()

	// 测试 InspectSession
	sessionID := "session-1778655608534012800"
	inspected, err := sessionStore.InspectSession(nil, sessionID)
	if err != nil {
		fmt.Printf("InspectSession error: %v\n", err)
	} else {
		fmt.Printf("Session found: %+v\n", inspected.Session)
		fmt.Printf("Conversation ID in metadata: %s\n", inspected.RequestID)
	}
}