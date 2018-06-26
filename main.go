package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/joho/godotenv"
)

type Block struct {
	Index     int
	Timestamp string
	BPM       int
	Hash      string
	PrevHash  string
}

var Blockchain []Block

func calculateHash(block Block) string {
	record := string(block.Index) + block.Timestamp + string(block.BPM) + block.PrevHash
	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

func generateBlock(oldBlock Block, BPM int) (Block, error) {

	var newBlock Block

	t := time.Now()

	newBlock.Index = oldBlock.Index + 1
	newBlock.Timestamp = t.String()
	newBlock.BPM = BPM
	newBlock.PrevHash = oldBlock.Hash
	newBlock.Hash = calculateHash(newBlock)

	return newBlock, nil
}

func isBlockValid(newBlock, oldBlock Block) bool {
	if oldBlock.Index+1 != newBlock.Index {
		return false
	}

	if oldBlock.Hash != newBlock.PrevHash {
		return false
	}

	if calculateHash(newBlock) != newBlock.Hash {
		return false
	}

	return true
}

func replaceChain(newBlocks []Block) {
	if len(newBlocks) > len(Blockchain) {
		Blockchain = newBlocks
	}
}

// ブロックがきたら反応するチャンネルをもつブロックチェーンサーバーを定義する
var bcServer chan []Block

//コネクションをハンドリングする関数
// net.Connはネットワークの共通インターフェイス
// インターフェイスがコード最後部のserver.Accept()となっているので、ここでは9000に外部からアクセスがきた場合インターフェイスを表示する
func handleConn(conn net.Conn) {
	defer conn.Close()

	// 文字列書き込み用のインターフェイス
	io.WriteString(conn, "Enter a new BPM:")

	scanner := bufio.NewScanner(conn)

	// take in BPM from stdin and add it to blockchain after conducting necessary validation
	go func() {
		for scanner.Scan() {
			bpm, err := strconv.Atoi(scanner.Text())
			if err != nil {
				log.Printf("%v not a number: %v", scanner.Text(), err)
				continue
			}
			newBlock, err := generateBlock(Blockchain[len(Blockchain)-1], bpm)
			if err != nil {
				log.Println(err)
				continue
			}
			if isBlockValid(newBlock, Blockchain[len(Blockchain)-1]) {
				newBlockchain := append(Blockchain, newBlock)
				replaceChain(newBlockchain)
			}

			//ブロックチェーンを送信（自分自身であっても他のノードでも
			bcServer <- Blockchain
			io.WriteString(conn, "\nEnter a new BPM:")
		}
	}()

	// ブロードキャスト
	go func() {
		for {
			// 30秒ごとにブロックチェーンをコンソールに書き込み
			time.Sleep(30 * time.Second)
			output, err := json.Marshal(Blockchain)
			if err != nil {
				log.Fatal(err)
			}
			// 9000番ポートと繋がっているインターフェイス全てに現在のblockchain状況を書き込む
			io.WriteString(conn, string(output))
		}
	}()

	for _ = range bcServer {
		spew.Dump(Blockchain)
	}
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}

	// ブロックチェーンを受け取る用のチャネルを生成
	bcServer = make(chan []Block)

	t := time.Now()
	genesisBlock := Block{0, t.String(), 0, "", ""}
	spew.Dump(genesisBlock)
	Blockchain = append(Blockchain, genesisBlock)

	// TCPプロトコルでデータをやりとりするためのリスナーを9000ポートで作成
	server, err := net.Listen("tcp", ":"+os.Getenv("ADDR"))
	if err != nil {
		log.Fatal(err)
	}
	defer server.Close()

	for {
		conn, err := server.Accept()
		if err != nil {
			log.Fatal(err)
		}
		// ゴルーチンでポート9000へのインターフェイスが常時開かれている状態になっている
		// nc localhost 9000で異なるターミナルからインターフェイスに通信すると以下のgo handleConn(server.Accept())が非同期で実行されることになる
		go handleConn(conn)
	}
}
