# Cocktails mit Fyne

Aus dem Projektordner mit `go run ./cocktails` starten. Standardmäßig verwendet
die App den Server unter `http://127.0.0.1:8080`. Mit `COCKTAILS_URL` lässt sich
die Adresse ändern.

Die Navigation links öffnet Cocktails, Zutaten, Kühlschrank, mögliche Rezepte,
Einkaufsliste und den Live-Feed. Die Suche oben lässt sich mit Enter ausführen.
Im Rezept kann man eine Bewertung abgeben. Der Feed öffnet ein eigenes Fenster,
das beim Bewerten geöffnet bleiben kann.

Die Bewertungsfunktionen benötigen eine erreichbare MongoDB auf der Serverseite.
Für den Live-Stream braucht der Server außerdem ein MongoDB-Replica-Set.

MVC-Aufteilung: `model.go` beschreibt die API-Daten und den HTTP-/SSE-Zugriff,
`view.go` baut die Fyne-Oberfläche, `controller.go` verbindet Aktionen,
Navigation und Feed mit dem Modell.
