package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/dengsgo/math-engine/engine"
)

type Task struct {
	Id        int       `json:"id"`
	AgentId   int       `json:"agentid"`
	Expr      string    `json:"expr"`
	Result    float64   `json:"result"`
	Status    string    `json:"status"`
	BeginDate time.Time `json:"begindate"`
	EndDate   time.Time `json:"enddate"`
	mutex     sync.Mutex
}

type Agent struct {
	Id   int    `json:"id"`
	Ip   string `json:"ip"`
	Port int    `json:"port"`
}

type Orchestrator struct {
	Ip   string `json:"ip"`
	Port int    `json:"port"`
}

var agent Agent
var orchestrator Orchestrator
var maxTasks int                   //максимальное количество одновременно выполняемых задач
var RegisteredTaskMap map[int]Task //хранилище зарегистрированных задач

func readSettings(settingsFile string) {
	var numLine = 0
	f, err := os.OpenFile(settingsFile, os.O_RDONLY, os.ModePerm)
	if err != nil {
		fmt.Println("error opening setting file!")
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		str := sc.Text() // GET the line string
		//в нулевой линии записаны настройки оркестратора, в первой агента
		switch numLine {
		case 0:
			//применяем настройки агента, записанные в файле
			err := json.Unmarshal([]byte(str), &orchestrator)
			if err != nil {
				fmt.Println("error unmarshal settings!")
				return
			}
			fmt.Println(orchestrator)
		case 1:
			//применяем настройки агента, записанные в файле
			err := json.Unmarshal([]byte(str), &agent)
			if err != nil {
				fmt.Println("error unmarshal settings!")
				return
			}
			fmt.Println(agent)
		}
		numLine++
	}
	if err := sc.Err(); err != nil {
		fmt.Println("error scanning setting file!")
		return
	}
}

func registerAgent() bool {
	payloadBuf := new(bytes.Buffer)
	json.NewEncoder(payloadBuf).Encode(agent)
	fmt.Println(payloadBuf)

	req, _ := http.NewRequest("POST", orchestrator.Ip+":"+strconv.Itoa(orchestrator.Port)+"/agent_reg/", payloadBuf)
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return false
	}

	defer res.Body.Close()
	fmt.Println("response Status:", res.Status)
	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return false
	}
	fmt.Println(string(body))
	//применяем настройки агента, записанные в файле
	err = json.Unmarshal([]byte(body), &agent)
	if err != nil {
		fmt.Println("error unmarshal body by register agent!")
		return false
	}
	fmt.Println(agent)
	return true
}

// проверка соединения с сервером
func checkAlive(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("I'm alive"))
}

// POST запрос оркестратор отправляет агенту задачу
func sendTask(w http.ResponseWriter, r *http.Request) {
	//Методом POST передается задача для вычисления
	if r.Method != http.MethodPost { //если это не POST
		fmt.Println("method is no POST!")
		w.WriteHeader(http.StatusBadRequest) //400
		w.Write([]byte("StatusBadRequest"))
		return
	}

	if len(RegisteredTaskMap) >= maxTasks { //если количество обрабатываемых задач больше максимального
		fmt.Println("too many handling task!")
		w.WriteHeader(http.StatusBadRequest) //400
		w.Write([]byte("Боливар так много не увезет!"))
		return
	}

	var task Task
	err := json.NewDecoder(r.Body).Decode(&task)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fmt.Println(task)
	task.Status = "in_progress"
	task.AgentId = agent.Id
	task.mutex.Lock()
	RegisteredTaskMap[task.Id] = task //добавим задачу в хранилище задач
	task.mutex.Unlock()
	//В ответе отсылаем ID агента, статус
	js, err := json.Marshal(task)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
	fmt.Printf("%+v\n", task)
	//начинаем решать задачу
	go solveTask(task)
}

func solveTask(task Task) { //здесь решаем строку задания
	time.Sleep(30 * time.Second) //имитируем длительную задачу
	toks, err := engine.Parse(task.Expr)
	if err != nil {
		fmt.Println("ERROR: " + err.Error())
		task.EndDate = time.Now()
		task.Status = "error"
		RegisteredTaskMap[task.Id] = task
		fmt.Println(RegisteredTaskMap)
		return
	}
	// []token -> AST Tree
	ast := engine.NewAST(toks, task.Expr)
	if ast.Err != nil {
		fmt.Println("ERROR: " + ast.Err.Error())
		task.EndDate = time.Now()
		task.Status = "error"
		RegisteredTaskMap[task.Id] = task
		fmt.Println(RegisteredTaskMap)
		return
	}
	// AST builder
	ar := ast.ParseExpression()
	if ast.Err != nil {
		fmt.Println("ERROR: " + ast.Err.Error())
		task.EndDate = time.Now()
		task.Status = "error"
		RegisteredTaskMap[task.Id] = task
		fmt.Println(RegisteredTaskMap)
		return
	}
	fmt.Printf("ExprAST: %+v\n", ar)
	// AST traversal -> result
	r := engine.ExprASTResult(ar)
	fmt.Println("progressing ...\t", r)
	fmt.Printf("%s = %v\n", task.Expr, r)
	task.Result = r
	task.EndDate = time.Now()
	task.Status = "finish"
	RegisteredTaskMap[task.Id] = task
	fmt.Println(RegisteredTaskMap)
}

// GET запрос получение задачи с ответом
func getFinishTask(w http.ResponseWriter, r *http.Request) {
	var mutex sync.Mutex
	idtask := r.URL.Query().Get("idtask")
	i, err := strconv.Atoi(idtask)
	if err != nil {
		fmt.Println("invalid ID task!")
		w.WriteHeader(http.StatusBadRequest) //400
		w.Write([]byte("invalid ID task"))
		return
	}
	if task, ok := RegisteredTaskMap[i]; ok { //если есть задача с таким ID возвращаем ее в ответе, удаляем из хранилища
		//если еще в процессе решения
		if task.Status == "in_progress" {
			fmt.Println("invalid ID task!")
			w.WriteHeader(http.StatusBadRequest) //400
			w.Write([]byte("still in progress"))
			return
		}
		//В ответе JSON с ID нужной задачи
		js, err := json.Marshal(task)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(js)
		mutex.Lock()
		delete(RegisteredTaskMap, i)
		mutex.Unlock()
		fmt.Println(RegisteredTaskMap)
		return
	} else {
		fmt.Println("have not task with idtask = " + idtask + "!")
		w.WriteHeader(http.StatusBadRequest) //400
		w.Write([]byte("have not task with idtask = " + idtask + "!"))
		return
	}
}

func main() {
	RegisteredTaskMap = make(map[int]Task)
	maxTasks = 2 //регулируем максимальное количество горутин у агента
	readSettings("settings.txt")
	if registerAgent() == false {
		fmt.Println("Not successful register agent!")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/check_alive/", checkAlive)        //проверка соединения с сервером
	mux.HandleFunc("/send_task/", sendTask)            //POST запрос оркестратор отправляет агенту задачу
	mux.HandleFunc("/get_finish_task/", getFinishTask) //GET запрос получение задачи с ответом
	http.ListenAndServe(":"+strconv.Itoa(agent.Port), mux)
}

/*
	GET запрос проверка на живучесть
http://localhost:8081/check_alive/
	POST запрос отправка задачи
{"id":1,"agentid":0,"expr":"1+6","result":0,"status":"start","begindate":"2024-02-07T19:25:34.10342208+03:00","enddate":"0001-01-01T00:00:00Z"}
http://localhost:8081/send_task/
	GET запрос получение задачи с ответом
http://localhost:8081/get_finish_task/?idtask=1
*/
