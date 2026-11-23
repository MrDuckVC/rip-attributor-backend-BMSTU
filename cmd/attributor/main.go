// Команда attributor запускает HTTP-сервис атрибуции.
package main

import "attributor/internal/api"

// main запускает сервер атрибуции.
func main() {
	api.StartServer()
}
