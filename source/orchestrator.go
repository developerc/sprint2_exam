package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

var IdAgent int                      //счетчик ID агента
var IdTask int                       //счетчик ID задач
var RegisteredAgentMap map[int]Agent //хранилище зарегистрированных агентов
var RegisteredTaskMap map[int]Task   //хранилище зарегистрированных задач
var TaskQueue []Task                 //очередь задач

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
	Id    int    `json:"id"`
	Ip    string `json:"ip"`
	Port  int    `json:"port"`
	mutex sync.Mutex
}

type AgentTask struct {
	AgentId int `json:"agentid"`
	TaskId  int `json:"taskid"`
}

type TaskDuration struct {
	IdTask   int `json:"idtask"`
	Duration int `json:"duration"`
}

func validExpr(exprStr string) bool {
	for _, c := range exprStr {
		if c >= '0' && c <= '9' || c >= '(' && c <= '+' || c == '-' || c == '*' || c == '/' || c == ' ' || c == '.' {
			continue
		} else {
			return false
		}
	}
	return true
}

func getIdResult(w http.ResponseWriter, r *http.Request) { //Получение результата по ID задачи
	if r.Method != http.MethodGet { //если это не GET
		fmt.Println("method is no GET!")
		w.WriteHeader(http.StatusBadRequest) //400
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write([]byte("StatusBadRequest"))
		return
	}
	id := r.URL.Query().Get("id")
	for _, task := range RegisteredTaskMap {
		i, err := strconv.Atoi(id)
		if err != nil {
			fmt.Println("invalid ID!")
			w.WriteHeader(http.StatusBadRequest) //400
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Write([]byte("invalid ID"))
			return
		}
		if i == task.Id {
			//В ответе JSON с ID нужной задачи
			js, err := json.Marshal(task)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Write(js)
			return
		}
	}
	fmt.Println("not fount task with ID")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusInternalServerError) //500
	w.Write([]byte("not fount task with ID"))
}

func getListTaskTime(w http.ResponseWriter, r *http.Request) { //обрабатываем GET запрос Получение списка незавершенных задач
	if r.Method != http.MethodGet { //если это не GET
		fmt.Println("method is no GET!")
		w.WriteHeader(http.StatusBadRequest) //400
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write([]byte("StatusBadRequest"))
		return
	}
	listTaskTime := make([]TaskDuration, 0)
	for _, task := range RegisteredTaskMap {
		if task.Status == "finish" {
			continue
		}
		taskDuration := TaskDuration{}
		diff := time.Now().Sub(task.BeginDate).Seconds()
		taskDuration.Duration = int(diff)
		taskDuration.IdTask = task.Id
		listTaskTime = append(listTaskTime, taskDuration)
	}
	//В ответе отсылаем ID_задачи : продолжительность_сек
	js, err := json.Marshal(listTaskTime)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(js)
}

func getAgentList(w http.ResponseWriter, r *http.Request) { //обрабатываем GET запрос на получение списка пар ID_агента : ID_задачи
	if r.Method != http.MethodGet { //если это не GET
		fmt.Println("method is no GET!")
		w.WriteHeader(http.StatusBadRequest) //400
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write([]byte("StatusBadRequest"))
		return
	}
	agentTaskList := make([]AgentTask, 0)
	for _, task := range RegisteredTaskMap {
		if task.Status == "in_progress" {
			agentTask := AgentTask{}
			agentTask.AgentId = task.AgentId
			agentTask.TaskId = task.Id
			agentTaskList = append(agentTaskList, agentTask)
		}

	}
	//В ответе отсылаем ID_агента : ID_задачи
	js, err := json.Marshal(agentTaskList)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(js)
}

func getTaskList(w http.ResponseWriter, r *http.Request) { //обрабатываем GET запрос на получение списка задач
	if r.Method != http.MethodGet { //если это не GET
		fmt.Println("method is no GET!")
		w.WriteHeader(http.StatusBadRequest) //400
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write([]byte("StatusBadRequest"))
		return
	}
	//В ответе отсылаем ID задачи
	js, err := json.Marshal(RegisteredTaskMap)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(js)
}

func handleExpr(w http.ResponseWriter, r *http.Request) { //обрабатываем принятый запрос с выражением
	//Методом POST передается выражение для вычисления
	if r.Method != http.MethodPost { //если это не POST
		fmt.Println("method is no POST!")
		w.WriteHeader(http.StatusBadRequest) //400
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write([]byte("StatusBadRequest"))
		return
	}
	expr := r.URL.Query().Get("expr")
	if validExpr(expr) == false { //если выражение содержит не валидный символ
		fmt.Println("expression have not valid symbol!")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusBadRequest) //400
		return
	}
	//Приняли выражение начинаем обрабатывать Ставим в очередь
	var task Task
	task.mutex.Lock()
	IdTask++
	task.Id = IdTask
	task.Expr = expr
	task.Status = "start"
	task.BeginDate = time.Now()
	TaskQueue = append(TaskQueue, task)
	RegisteredTaskMap[task.Id] = task
	task.mutex.Unlock()
	//В ответе отсылаем ID задачи
	js, err := json.Marshal(task)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(js)
	fmt.Println(expr)

}

func agentReg(w http.ResponseWriter, r *http.Request) { //регистрация нового агента у оркестратора
	var agent Agent
	err := json.NewDecoder(r.Body).Decode(&agent)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// увеличим счетчик агентов, добавим в мар
	agent.mutex.Lock()
	IdAgent++
	agent.Id = IdAgent
	RegisteredAgentMap[agent.Id] = agent
	agent.mutex.Unlock()

	//В ответе отсылаем ID агента
	js, err := json.Marshal(agent)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
	fmt.Printf("%+v\n", agent)
}

// Обработчик тестового запроса.
func home(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write([]byte("Ответ от Orchestrator"))
}

// обработчик очереди задач
func handlerTaskQueue() {
	var mutex sync.Mutex
	for {
		if len(TaskQueue) > 0 { //если в очереди есть задачи, начинаем работу
			if len(RegisteredAgentMap) > 0 { //если есть зарегистрированные агенты
				fmt.Println(TaskQueue, RegisteredAgentMap)
				for _, agent := range RegisteredAgentMap {
					if task, ok := sendTask(TaskQueue[0], agent); ok { //отправляем задачу агенту. Если агент принял, изменяем состояние задачи в хранилище
						mutex.Lock()
						RegisteredTaskMap[task.Id] = task
						mutex.Unlock()
						TaskQueue = TaskQueue[1:]
						break
					}
				}
			}
		}
		time.Sleep(1 * time.Second)
	}
}

// отсылаем задачу агенту
func sendTask(task Task, agent Agent) (Task, bool) {
	payloadBuf := new(bytes.Buffer)
	json.NewEncoder(payloadBuf).Encode(task)
	fmt.Println(payloadBuf)

	req, _ := http.NewRequest("POST", agent.Ip+":"+strconv.Itoa(agent.Port)+"/send_task/", payloadBuf)
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return task, false
	}

	defer res.Body.Close()
	fmt.Println("response Status:", res.Status)
	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return task, false
	}
	fmt.Println(string(body))
	if res.StatusCode == 200 {
		//применяем настройки агента, записанные в файле
		err = json.Unmarshal([]byte(body), &task)
		if err != nil {
			fmt.Println("error unmarshal body by register agent!")
			return task, false
		}
		fmt.Println(task)
		return task, true
	}
	return task, false
}

// проверяем решены ли задачи
func checkFinishTask() {
	var mutex sync.Mutex
	for {
		for _, task := range RegisteredTaskMap {
			if task.Status == "in_progress" { //если задача находится на обработке у агента
				if resTask, ok := getTaskFromAgent(task); ok {
					mutex.Lock()
					RegisteredTaskMap[resTask.Id] = resTask
					mutex.Unlock()
				}
			}
		}

		time.Sleep(1 * time.Second)
	}
}

// получаем задачу от агента
func getTaskFromAgent(task Task) (Task, bool) {
	//находим какой агент решает задачу
	agent := RegisteredAgentMap[task.AgentId]

	req, _ := http.NewRequest("POST", agent.Ip+":"+strconv.Itoa(agent.Port)+"/get_finish_task/?idtask="+strconv.Itoa(task.Id), nil)
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return task, false
	}

	defer res.Body.Close()
	fmt.Println("response Status:", res.Status)
	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return task, false
	}
	fmt.Println(string(body))

	if res.StatusCode == 200 {
		//применяем настройки агента, записанные в файле
		err = json.Unmarshal([]byte(body), &task)
		if err != nil {
			fmt.Println("error unmarshal body by register agent!")
			return task, false
		}
		fmt.Println(task)
		return task, true
	}
	return task, false
}

func main() {
	TaskQueue = make([]Task, 0)
	RegisteredTaskMap = make(map[int]Task)
	RegisteredAgentMap = make(map[int]Agent)
	go handlerTaskQueue() //обрабатываем очередь задач в отдельной горутине
	go checkFinishTask()  //проверяем готовность решения задач в отдельной горутине
	mux := http.NewServeMux()
	mux.HandleFunc("/", home)                               //проверка соединения с сервером
	mux.HandleFunc("/send_expr/", handleExpr)               //POST Запрос отправки вычисления выражения
	mux.HandleFunc("/agent_reg/", agentReg)                 //POST запрос регистрация агента у оркестратора
	mux.HandleFunc("/get_task_list/", getTaskList)          //получение списка задач у оркестратора
	mux.HandleFunc("/get_agent_list/", getAgentList)        //получение списка агентов с выполняемыми операциями
	mux.HandleFunc("/get_list_task_time/", getListTaskTime) //Получение списка незавершенных задач в виде пар ID_задачи : время выполнения
	mux.HandleFunc("/get_id_result/", getIdResult)          //Получение результата по ID задачи
	http.ListenAndServe(":8080", mux)
}

/*
	POST Запрос отправки вычисления выражения
http://localhost:8080/send_expr/?expr=1%2B6
//вместо плюса отправляем %2B
	POST запрос регистрация агента у оркестратора
http://localhost:8080/agent_reg/
{"ip":"localhost","port":8081}
"Content-Type", "application/json"
	GET запрос Получение списка задач у оркестратора
http://localhost:8080/get_task_list/
	GET запрос Получение списка вычислительных мощностей в виде списка пар ID_агента : ID_задачи
http://localhost:8080/get_agent_list/
	GET запрос Получение списка незавершенных задач в виде пар ID_задачи : время выполнения
http://localhost:8080/get_list_task_time/
	GET запрос Получение результата по ID задачи
http://localhost:8080/get_id_result/?id=1
*/
