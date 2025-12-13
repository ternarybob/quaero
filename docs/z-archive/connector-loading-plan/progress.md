# Progress

| Task | Status | Notes |
|------|--------|-------|
| 1 | complete | Added GitLab ConnectorType to models |
| 2 | complete | Created LoadConnectorsFromFiles in load_connectors.go |
| 3 | complete | Added interface method to StorageManager |
| 4 | complete | Added Manager.LoadConnectorsFromFiles method |
| 5 | complete | Called loader in app.go initDatabase |
| 6 | complete | Warning logging included in Task 2 |
| 7 | complete | Created API tests in test/api/connector_loading_test.go |
| 8 | complete | Created UI tests in test/ui/connector_loading_test.go |

Deps: [x] 1 -> [x] 2 -> [x] 3,4 -> [x] 5 -> [x] 6 -> [x] 7,8

## Validation
- go build ./... : SUCCESS
- All tasks complete
