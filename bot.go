package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api"
)

func keyboardFromSearchResults(results []string) (tgbotapi.InlineKeyboardMarkup, bool) {
	var keyboard tgbotapi.InlineKeyboardMarkup
	tooLong := len(results) > 10
	if tooLong {
		results = results[:11]
	}
	for _, name := range results {
		button := tgbotapi.NewInlineKeyboardButtonData(name, name)
		keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, []tgbotapi.InlineKeyboardButton{button})
	}
	return keyboard, tooLong
}

func sendPhoto(bot *tgbotapi.BotAPI, chatID int64, link, caption string) (tgbotapi.APIResponse, error) {
	params := url.Values{
		"chat_id":[]string{strconv.FormatInt(chatID, 10)},
		"photo":[]string{link},
		"caption":[]string{caption},
		"parse_mode":[]string{"MarkdownV2"},
	}
	return bot.MakeRequest("sendPhoto", params)
}

var (
	statNames = [5]string{
		"Знания",
		"Умение преподавать",
		"В общении",
		"«Халявность»",
		"Общая оценка",
	}
)

var TOKEN = os.Getenv("WIKIMIPT_TGTOKEN")

func main() {
	bot, err := tgbotapi.NewBotAPI(TOKEN)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = false

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.CallbackQuery != nil {
			profile, err := getProfile(update.CallbackQuery.Data)
			if err != nil {
				log.Panic(err)
			} else {
				text := "*"+profile.name+"*\n\n"+profile.desc
				for index, stat := range profile.stats {
					text += "_*" + statNames[index] + "*_:\n"
					if stat.votes != 0 {
						for i := 0; i < 5; i++ {
							if i < int(stat.value) {
								text += "\U00002B50"
							} else {
								text += "\U0001F311"
							}
						}
						text += escapeMarkdownV2(fmt.Sprintf(" %4.2f (%d голосов)\n", stat.value, stat.votes))
					} else {
						for i := 0; i < 5; i++ {text += "\U0001F31A"}
						text += escapeMarkdownV2(" (нет голосов)\n")
					}
				}
				text += fmt.Sprintf("[_Страница на wikimipt_](%s)", strings.ReplaceAll(pageUrl(profile.name), ")", "\\)"))
				_, err = sendPhoto(bot, update.CallbackQuery.Message.Chat.ID, profile.photo, text)
			}
		}
		if update.Message != nil && update.Message.Text != ""{
			if update.Message.IsCommand() {
				switch update.Message.Command() {
				case "start": {
					text := "Привет! Я ищу преподавателей на wikimipt.org. Попробуй написать мне фамилию преподавателя."
					if _, err := bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, text)); err != nil {
						log.Panic(err)
					}
				}
				default: {
					text := fmt.Sprintf("Неизвестная команда: %s", update.Message.Text)
					if _, err := bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, text)); err != nil {
						log.Panic(err)
					}
				}
				}
			} else {
				var msg tgbotapi.MessageConfig

				results, err := search(update.Message.Text)
				if err == badCharError {
					msg = tgbotapi.NewMessage(update.Message.Chat.ID, "Неподдерживаемые символы в запросе")
				} else if err != nil {
					log.Panic(err)
				} else {
					if len(results) != 0 {
						msg = tgbotapi.NewMessage(update.Message.Chat.ID, "Результаты поиска:")
						msg.ParseMode = "MarkdownV2"
						msg.ReplyToMessageID = update.Message.MessageID
						var tooLong bool
						msg.ReplyMarkup, tooLong = keyboardFromSearchResults(results)
						if tooLong {
							msg.Text += "\n_\\(первые 10 результатов\\)_"
						}
					} else {
						msg = tgbotapi.NewMessage(update.Message.Chat.ID, "Ничего не найдено")
						msg.ReplyToMessageID = update.Message.MessageID
					}
				}
				if _, err := bot.Send(msg); err != nil {
					log.Panic(err)
				}
			}
		}
	}
}