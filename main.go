package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const bin = "400000"

const initialScreenText = "1. Create an account\n2. Log into account\n0. Exit"
const createCardText = "Your card has been created"
const yourCartNumText = "Your card number:"
const yourPinText = "Your card PIN:"
const enterCardNumText = "Enter your card number:"
const enterPinText = "Enter your PIN:"
const successLogInText = "You have successfully logged in!"

type Card struct {
	gorm.Model
	Number  string `gorm:"type:text"`
	PIN     string `gorm:"type:text"`
	Balance int    `gorm:"default:0"`
}

func cmdRequest() string {
	fileNamePt := flag.String("fileName", "db.s3db", "Enter the filename of the database")

	// After declaring all the flags, enable command-line flag parsing:
	flag.Parse()
	return *fileNamePt
}

func readInput() string {
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		log.Fatal(err)
	}
	return strings.TrimSpace(input) // Remove any leading/trailing whitespace
}

func createCardDb(db *gorm.DB) Card {
	var partialCardNum string
	var checkSum string
	var cardNum string
	var pin string

	partialCardNum = bin + randomNum(9)
	check, ok := generateLuhnCheckSum(partialCardNum, 15)
	if ok {
		checkSum = check
		cardNum = partialCardNum + checkSum
	}
	pin = randomNum(4)

	// Create a new card and save it to the database
	newCard := Card{
		Number:  cardNum,
		PIN:     pin,
		Balance: 0,
	}
	db.Create(&newCard)

	return newCard
}

func getCardByNumAndPinFromDb(db *gorm.DB, cardNum string, pin string) (Card, error) {
	var card Card
	result := db.Where("number = ?", cardNum).Where("pin = ?", pin).First(&card)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// Card not found with the given number and pin
			return Card{}, result.Error
		}
		// Other errors (e.g., database connection issues)
		return Card{}, result.Error
	}
	return card, nil
}

func getCardByNumFromDb(db *gorm.DB, cardNum string) (Card, error) {
	var card Card
	result := db.Where("number = ?", cardNum).First(&card)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// Card not found with the given number and pin
			return Card{}, result.Error
		}
		// Other errors (e.g., database connection issues)
		return Card{}, result.Error
	}
	return card, nil
}

func cardScreen(db *gorm.DB, card *Card) bool {
	var choice string
Loop:
	for {
		fmt.Println(`
		1. Balance
		2. Add income
		3. Do transfer
		4. Close account
		5. Log out
		0. Exit`)
		choice = readInput()
		fmt.Println("choice =", choice)

		switch choice {
		case "1":
			fmt.Printf("Balance: %d\n", card.Balance)
			continue Loop
		case "2":
			fmt.Println("Enter income:")
			incomeStr := readInput()
			income, err := strconv.Atoi(incomeStr)
			if err != nil {
				fmt.Println("Invalid input for income")
				continue Loop
			}
			card.Balance += income
			db.Save(&card)
			fmt.Println("Income was added!")
			continue Loop
		case "3":
			fmt.Println("Transfer")
			fmt.Println(enterCardNumText)
			var cardNum string = readInput()

			// If the receiver's card number doesn't pass the Luhn algorithm, you should output:
			// Probably you made a mistake in the card number. Please try again!
			if !isValidLuhnAlgo(cardNum) {
				fmt.Println("Probably you made a mistake in the card number. Please try again!")
				break
			}

			// If the user tries to transfer money to the same account, output the following message:
			// You can't transfer money to the same account!
			if cardNum == card.Number {
				fmt.Println("You can't transfer money to the same account!")
				break
			}

			// If the receiver's card number doesn't exist, you should output: Such a card does not exist.
			receiverCard, err := getCardByNumFromDb(db, cardNum)
			if err != nil {
				fmt.Println("Such a card does not exist.")
				break
			}

			// If there is no error, ask the user how much money they want to transfer and make the transaction.
			fmt.Println("Enter how much money you want to transfer:")
			transferAmountStr := readInput()
			transferAmount, err := strconv.Atoi(transferAmountStr)
			if err != nil {
				fmt.Println("Invalid transfer amount")
				continue Loop
			}
			// If the user tries to transfer more money than they have, output: Not enough money!
			if transferAmount > card.Balance {
				fmt.Println("Not enough money!")
				break Loop
			}

			card.Balance -= transferAmount
			receiverCard.Balance += transferAmount
			db.Save(card)
			db.Save(&receiverCard)

			fmt.Println("Success!")
			continue Loop

		case "4":
			db.Delete(&card)
			continue Loop
		case "5":
			fmt.Println("You have successfully logged out!")
			break Loop
		case "0":
			return true
		default:
			fmt.Println("Invalid option, please try again.")
			continue Loop
		}
	}
	return false
}

func mainMenu(db *gorm.DB) {

MenuLoop:
	for {
		fmt.Println(initialScreenText)
		choiceStr := readInput()
		choice, err := strconv.Atoi(choiceStr)
		if err != nil {
			fmt.Println("Invalid choice")
			continue
		}
		switch choice {
		case 1:
			fmt.Println(createCardText)

			newCard := createCardDb(db)

			fmt.Println(yourCartNumText)
			fmt.Println(newCard.Number)
			fmt.Println(yourPinText)
			fmt.Println(newCard.PIN)

		case 2:
			fmt.Println(enterCardNumText)
			var enteredCardNum string = readInput()
			fmt.Println(enterPinText)
			var enteredPin string = readInput()

			// Get card db instance based on enterdCardNum and enteredPinNum
			card, err := getCardByNumAndPinFromDb(db, enteredCardNum, enteredPin)
			if err != nil {
				fmt.Printf("cannot retrieve Card: %v\n", err)
				break
			}
			fmt.Println(successLogInText)
			isExited := cardScreen(db, &card)
			if isExited {
				break MenuLoop
			}
		case 0:
			break MenuLoop
		}
	}
}

func main() {

	// command line logic
	dbName := cmdRequest()

	// GROM Logic
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	// Migrate the schema
	err = db.AutoMigrate(&Card{})
	if err != nil {
		log.Fatalf("Failed to auto migrate: %v", err)
	}

	// Main menu interaction
	mainMenu(db)

	fmt.Println("Bye!")
}

func isValidLuhnAlgo(cardNum string) bool {
	// Get last string
	runes := []rune(cardNum)
	lastCharStr := string(runes[len(runes)-1])
	upToLastStr := string(runes[:len(runes)-1])

	checkSum, ok := generateLuhnCheckSum(upToLastStr, len(upToLastStr))
	if !ok || checkSum != lastCharStr {
		return false
	}
	return true
}

func generateLuhnCheckSum(partialCardNum string, cardNumLen int) (string, bool) {
	// check if cardNum is numeric
	if !isNumericCardNumber(partialCardNum) {
		return "", false
	}

	// check length it should be cadNumLen
	if len(partialCardNum) != cardNumLen {
		return "", false
	}
	// fmt.Println(partialCardNum)

	// Mutliply odd digits by 2 and Subtract 9 from those which will be more than 9
	var partialCardNumIntArray = make([]int, cardNumLen)
	sum := 0
	for i, runeVal := range partialCardNum {
		intNum := int(runeVal - '0')
		// fmt.Print(intNum)
		if i%2 == 0 {
			intNum *= 2
			if intNum > 9 {
				intNum -= 9
			}
		}
		partialCardNumIntArray[i] = intNum
		sum += intNum
	}
	// fmt.Println()
	// fmt.Println(partialCardNumIntArray)

	// Get module of sum and subtract the module from 10
	checkSum := "0"
	mod := sum % 10
	if mod == 0 {
		checkSum = "0"
	} else {
		checkSum = strconv.Itoa(10 - mod)
	}

	return checkSum, true
}

func randomNum(len int) string {
	rand.Seed(time.Now().UnixNano())
	var randomNumStr string
	for i := 0; i < len; i++ {
		randomNumStr += strconv.Itoa(rand.Intn(10))
	}

	return randomNumStr
}

func isNumericCardNumber(num string) bool {
	// Regex for integer or floating point numbers
	match, _ := regexp.MatchString("\\d+", num)
	return match
}
