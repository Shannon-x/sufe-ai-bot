package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"
	
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// addMentionGreeting adds a friendly greeting when triggered by mention word
func (h *MessageHandler) addMentionGreeting(message, mentionWord string, update *tgbotapi.Update) string {
	// 获取机器人性格设置
	personality := h.config.Context.BotPersonality
	if personality == "" {
		personality = "cute" // 默认可爱性格
	}
	// 根据时间段和性格选择不同风格的问候
	hour := time.Now().Hour()
	var greetings []string
	
	if hour >= 5 && hour < 9 {
		// 早晨问候
		switch personality {
		case "professional":
			greetings = []string{
				"早上好，新的一天开始了。有什么可以帮助您的吗？",
				"早安，我已准备好为您服务。",
				"早上好，请问有什么需要协助的？",
			}
		case "humorous":
			greetings = []string{
				"早起的鸟儿有虫吃，早起的你有AI陪！😄",
				"哇！这么早就起来了？是太阳从西边出来了吗？",
				"早安！咖啡喝了吗？我已经充满电了！⚡",
			}
		case "warm":
			greetings = []string{
				"早上好！希望你今天有个美好的开始～",
				"早安，新的一天充满希望！有什么想聊的吗？",
				"早晨好！阳光正好，心情也要美美的哦～",
			}
		default: // cute
			greetings = []string{
				"早上好呀！新的一天，有什么可以帮到你的吗？☀️",
				"早安！我在这里，请问需要什么帮助呢？🌅",
				"美好的早晨！有什么问题尽管问我哦～",
				"早上好！今天想聊点什么呢？😊",
				"新的一天开始了！我能为你做些什么？🌸",
			}
		}
	} else if hour >= 9 && hour < 12 {
		// 上午问候
		switch personality {
		case "professional":
			greetings = []string{
				"上午好，有什么可以为您服务的吗？",
				"您好，上午时间充裕，请问需要什么帮助？",
				"上午好，我已准备好协助您处理问题。",
			}
		case "humorous":
			greetings = []string{
				"上午好！咖啡喝了吗？我已经满血复活啦！☕",
				"哟！工作时间摸鱼找我聊天？我懂的～😏",
				"上午好！让我们一起愉快地度过这段时光吧！",
			}
		case "warm":
			greetings = []string{
				"上午好！希望你今天状态满满～有什么想聊的？",
				"你好呀！上午的阳光很好，心情也要美美的哦～",
				"上午好！有什么可以帮到你的吗？我一直在这里～",
			}
		default: // cute
			greetings = []string{
				"你好呀！有什么可以帮助你的吗？😊",
				"嗨～我在这里！需要什么帮助呢？",
				"你好！有什么问题想问我吗？🌟",
				"叮咚～有人找我吗？请说～",
				"在的在的！有什么可以效劳的？✨",
			}
		}
	} else if hour >= 12 && hour < 14 {
		// 午间问候
		switch personality {
		case "professional":
			greetings = []string{
				"午安，午休时间有什么可以帮助您的吗？",
				"中午好，请问有什么需要协助的？",
				"午间好，我在这里为您服务。",
			}
		case "humorous":
			greetings = []string{
				"午饭吃了吗？没吃的话先聊天填填肚子？😄",
				"午安！是不是吃饱了想找人聊天消食？",
				"中午好！让我猜猜，你是不是在等外卖？🍱",
			}
		case "warm":
			greetings = []string{
				"午安！记得好好吃饭哦～有什么想聊的吗？",
				"中午好！午休时光，放松一下吧～",
				"你好！希望你有个愉快的午餐时间～",
			}
		default: // cute
			greetings = []string{
				"午安！有什么可以帮到你的吗？🌞",
				"你好～午休时间，轻松聊聊？",
				"嗨！有什么想问的尽管说～😊",
				"我在呢！需要什么帮助？",
				"午间好！有什么可以为你解答的？",
			}
		}
	} else if hour >= 14 && hour < 18 {
		// 下午问候
		switch personality {
		case "professional":
			greetings = []string{
				"下午好，有什么工作上的问题需要帮助吗？",
				"您好，下午时光，请问有什么可以协助的？",
				"下午好，我随时准备为您解答问题。",
			}
		case "humorous":
			greetings = []string{
				"下午好！是不是犯困了来找我提神？😆",
				"嗨！下午茶时间到，聊天配茶更香哦～☕",
				"下午好！让我们一起战胜困意吧！💪",
			}
		case "warm":
			greetings = []string{
				"下午好！工作累了吧？休息一下聊聊天～",
				"你好！下午的时光总是过得很快呢～",
				"下午好！有什么烦恼可以跟我说说哦～",
			}
		default: // cute
			greetings = []string{
				"下午好！有什么可以帮助你的吗？☕",
				"你好呀！下午时光，有什么想聊的？",
				"嗨～我在这里，有问题尽管问！",
				"叮～有人呼叫我吗？请讲～😊",
				"下午好！需要什么帮助呢？🌤️",
			}
		}
	} else if hour >= 18 && hour < 22 {
		// 晚间问候
		switch personality {
		case "professional":
			greetings = []string{
				"晚上好，今天辛苦了。有什么可以帮助您的吗？",
				"晚安，请问有什么需要协助的？",
				"晚上好，我在这里随时为您服务。",
			}
		case "humorous":
			greetings = []string{
				"晚上好！是来找我聊天解闷的吗？🌙",
				"嗨！夜生活开始了，有什么精彩的事要分享吗？",
				"晚上好！让我们一起度过这个美好的夜晚吧！✨",
			}
		case "warm":
			greetings = []string{
				"晚上好！今天过得怎么样？有什么想聊的吗？",
				"晚安～累了一天了吧？我在这里陪你聊天～",
				"你好！晚上是放松的好时光，有什么烦恼都可以说给我听～",
			}
		default: // cute
			greetings = []string{
				"晚上好！有什么可以帮到你的吗？🌙",
				"晚安～有什么想问的吗？",
				"你好！晚间时光，需要什么帮助？✨",
				"嗨～我在这里，有什么可以效劳的？",
				"晚上好呀！有问题尽管问我～😊",
			}
		}
	} else {
		// 深夜问候
		switch personality {
		case "professional":
			greetings = []string{
				"深夜了，您还在工作吗？有什么可以帮助的？",
				"夜深了，请问有什么紧急的事情需要协助？",
				"深夜好，我依然在这里为您服务。",
			}
		case "humorous":
			greetings = []string{
				"哇！夜猫子出没！是失眠了还是在修仙？🦉",
				"深夜好！是不是刷手机刷到我这里来了？😄",
				"这么晚了还不睡？来来来，让我讲个笑话助眠～",
			}
		case "warm":
			greetings = []string{
				"夜深了，还没休息吗？有什么心事可以跟我说说～",
				"深夜好！睡不着的话，我陪你聊聊天吧～",
				"这么晚了，要注意休息哦～有什么想聊的吗？",
			}
		default: // cute
			greetings = []string{
				"夜深了，有什么可以帮助你的吗？🌙",
				"还没睡呀？有什么想问的吗？",
				"深夜好！我一直在这里～",
				"夜猫子你好！需要什么帮助？🦉",
				"这么晚了，有什么可以为你解答的？💫",
			}
		}
	}
	
	// 根据性格添加通用问候
	var generalGreetings []string
	switch personality {
	case "professional":
		generalGreetings = []string{
			"您好，有什么可以帮助您的吗？",
			"您好，我在这里为您服务。",
			"收到您的消息，请问有什么需要帮助的？",
			"您好，我准备好为您解答问题了。",
			"在的，请问有什么可以协助您的？",
		}
	case "humorous":
		generalGreetings = []string{
			"哟！有人叫我？是要听个笑话吗？😄",
			"叮咚！外卖...哦不对，是AI助手到！",
			"嘿！被你发现我了～有啥好玩的事吗？",
			"报告！AI小助手前来报到！有何指示？",
			"哈喽！今天想聊点啥有趣的？🎭",
		}
	case "warm":
		generalGreetings = []string{
			"你好呀，很高兴见到你！有什么可以帮忙的吗？",
			"嗨，朋友！有什么想聊的吗？",
			"你好！我一直在这里陪着你呢～",
			"很高兴收到你的消息！需要什么帮助吗？",
			"你来啦！有什么可以为你做的吗？温暖的拥抱～",
		}
	default: // cute
		generalGreetings = []string{
			"叮咚～有人在找我吗？😊",
			"嗨嗨！我来啦，有什么可以帮忙的？",
			"你好呀！有什么想问的尽管说～",
			"在的在的！需要什么帮助呢？✨",
			"嘿～听到有人叫我！怎么啦？",
			"我在这里哦！有什么可以效劳的？",
			"叮～收到呼叫！请问有什么需要帮助的？",
			"你好！我准备好回答你的问题啦～",
			"嗨呀！有什么想聊的吗？😊",
			"来啦来啦！有什么可以帮到你的？",
		}
	}
	
	// 合并问候语
	allGreetings := append(greetings, generalGreetings...)
	
	// 获取最近使用的问候语历史
	var recentGreetings []int
	recentGreetingsKey := fmt.Sprintf("recent_greetings_%d", update.Message.Chat.ID)
	recentGreetingsStr, err := h.storage.GetUserState(context.Background(), update.Message.From.ID, recentGreetingsKey)
	if err == nil && recentGreetingsStr != "" {
		json.Unmarshal([]byte(recentGreetingsStr), &recentGreetings)
	}
	
	// 创建候选索引列表（排除最近使用的）
	candidateIndices := []int{}
	for i := 0; i < len(allGreetings); i++ {
		isRecent := false
		for _, recentIdx := range recentGreetings {
			if i == recentIdx {
				isRecent = true
				break
			}
		}
		if !isRecent {
			candidateIndices = append(candidateIndices, i)
		}
	}
	
	// 如果所有问候语都最近使用过，清空历史
	if len(candidateIndices) == 0 {
		recentGreetings = []int{}
		for i := 0; i < len(allGreetings); i++ {
			candidateIndices = append(candidateIndices, i)
		}
	}
	
	// 随机选择一个问候语
	rand.Seed(time.Now().UnixNano())
	selectedIdx := candidateIndices[rand.Intn(len(candidateIndices))]
	greeting := allGreetings[selectedIdx]
	
	// 更新最近使用的问候语历史（保留最近5个）
	recentGreetings = append(recentGreetings, selectedIdx)
	if len(recentGreetings) > 5 {
		recentGreetings = recentGreetings[len(recentGreetings)-5:]
	}
	
	// 保存更新后的历史
	updatedRecentGreetingsStr, _ := json.Marshal(recentGreetings)
	h.storage.SetUserState(context.Background(), update.Message.From.ID, recentGreetingsKey, string(updatedRecentGreetingsStr))
	
	// If the message only contains the mention word, just return the greeting
	trimmed := strings.TrimSpace(message)
	if strings.EqualFold(trimmed, mentionWord) || trimmed == "" {
		return greeting
	}
	
	// Otherwise, acknowledge the mention and process the message
	return fmt.Sprintf("%s\n\n关于你的问题：%s", greeting, message)
}