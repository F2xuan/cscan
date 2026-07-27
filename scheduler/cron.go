package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"github.com/zeromicro/go-zero/core/logx"
)

// cronParser 包级别Cron解析器（秒级精度），避免重复创建
var cronParser = cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// CronTask 定时任务
type CronTask struct {
	Id           string       `json:"id"`
	Name         string       `json:"name"`
	ScheduleType string       `json:"scheduleType"` // cron: Cron表达式, once: 指定时间执行一次
	CronSpec     string       `json:"cronSpec"`     // Cron表达式 (scheduleType=cron时使用)
	ScheduleTime string       `json:"scheduleTime"` // 指定执行时间 (scheduleType=once时使用)
	WorkspaceId  string       `json:"workspaceId"`
	MainTaskId   string       `json:"mainTaskId"` // 关联的任务ID
	TaskName     string       `json:"taskName"`   // 关联的任务名称
	Target       string       `json:"target"`     // 扫描目标（从任务复制）
	Config       string       `json:"config"`     // 任务配置（从任务复制）
	Status       string       `json:"status"`     // enable/disable
	LastRunTime  string       `json:"lastRunTime"`
	NextRunTime  string       `json:"nextRunTime"`
	EntryId      cron.EntryID `json:"-"`
	timer        *time.Timer  `json:"-"`
}

// CronTaskSource 定时任务数据源接口（用于从MongoDB加载）
type CronTaskSource interface {
	FindAllCronTasks(ctx context.Context) ([]CronTaskData, error)
	FindCronTaskByCronTaskId(ctx context.Context, cronTaskId string) (*CronTaskData, error)
	UpdateCronTaskRunInfo(ctx context.Context, cronTaskId string, lastRunTime, nextRunTime, status string) error
}

// CronTaskData 定时任务数据（从MongoDB读取的通用结构）
type CronTaskData struct {
	CronTaskId   string
	Name         string
	ScheduleType string
	CronSpec     string
	ScheduleTime string
	WorkspaceId  string
	MainTaskId   string
	TaskName     string
	Target       string
	Config       string
	Status       string
	LastRunTime  string
	NextRunTime  string
}

// CronManager 定时任务管理器
type CronManager struct {
	scheduler *Scheduler
	rdb       *redis.Client
	tasks     map[string]*CronTask
	cronKey   string
	taskSrc   CronTaskSource // MongoDB数据源
	mu        sync.RWMutex
	stopCh    chan struct{} // 优雅停止信号
}

// NewCronManager 创建定时任务管理器
func NewCronManager(scheduler *Scheduler, rdb *redis.Client, taskSrc CronTaskSource) *CronManager {
	return &CronManager{
		scheduler: scheduler,
		rdb:       rdb,
		tasks:     make(map[string]*CronTask),
		cronKey:   "cscan:cron:tasks",
		taskSrc:   taskSrc,
		stopCh:    make(chan struct{}),
	}
}

// LoadTasks 从MongoDB加载定时任务（主存储），同步到Redis缓存
func (m *CronManager) LoadTasks(ctx context.Context) error {
	if m.taskSrc != nil {
		// 优先从MongoDB加载
		taskDataList, err := m.taskSrc.FindAllCronTasks(ctx)
		if err == nil {
			logx.Infof("[CronManager] Loaded %d cron tasks from MongoDB", len(taskDataList))

			m.mu.Lock()
			for _, td := range taskDataList {
				task := &CronTask{
					Id:           td.CronTaskId,
					Name:         td.Name,
					ScheduleType: td.ScheduleType,
					CronSpec:     td.CronSpec,
					ScheduleTime: td.ScheduleTime,
					WorkspaceId:  td.WorkspaceId,
					MainTaskId:   td.MainTaskId,
					TaskName:     td.TaskName,
					Target:       td.Target,
					Config:       td.Config,
					Status:       td.Status,
					LastRunTime:  td.LastRunTime,
					NextRunTime:  td.NextRunTime,
				}
				if task.Status == "enable" {
					m.startTask(task)
				}
				m.tasks[task.Id] = task
			}
			m.mu.Unlock()

			// 同步到Redis缓存（无锁IO）
			for _, td := range taskDataList {
				task := &CronTask{
					Id: td.CronTaskId, Name: td.Name, ScheduleType: td.ScheduleType,
					CronSpec: td.CronSpec, ScheduleTime: td.ScheduleTime,
					WorkspaceId: td.WorkspaceId, MainTaskId: td.MainTaskId,
					TaskName: td.TaskName, Target: td.Target, Config: td.Config,
					Status: td.Status, LastRunTime: td.LastRunTime, NextRunTime: td.NextRunTime,
				}
				data, marshalErr := json.Marshal(task)
				if marshalErr != nil {
					logx.Errorf("[CronManager] Failed to marshal task for redis sync: cronTaskId=%s, err=%v", task.Id, marshalErr)
				} else {
					m.rdb.HSet(ctx, m.cronKey, task.Id, data)
				}
			}
			return nil
		}
		// MongoDB出错，尝试从Redis回退加载
		logx.Errorf("[CronManager] Failed to load from MongoDB, falling back to Redis: %v", err)
	}

	// 回退：从Redis加载
	data, err := m.rdb.HGetAll(ctx, m.cronKey).Result()
	if err != nil {
		return err
	}

	logx.Infof("[CronManager] Loaded %d cron tasks from Redis (fallback)", len(data))

	m.mu.Lock()
	for id, taskData := range data {
		var task CronTask
		if err := json.Unmarshal([]byte(taskData), &task); err != nil {
			continue
		}
		task.Id = id
		if task.Status == "enable" {
			m.startTask(&task)
		}
		m.tasks[id] = &task
	}
	m.mu.Unlock()

	return nil
}

// AddTask 添加定时任务
func (m *CronManager) AddTask(ctx context.Context, task *CronTask) error {
	if task.ScheduleType == "cron" {
		schedule, err := cronParser.Parse(task.CronSpec)
		if err != nil {
			return fmt.Errorf("invalid cron spec: %v", err)
		}
		task.NextRunTime = schedule.Next(time.Now()).Local().Format("2006-01-02 15:04:05")
	} else if task.ScheduleType == "once" {
		if task.ScheduleTime == "" {
			return fmt.Errorf("schedule_time is required for once type")
		}
		task.NextRunTime = task.ScheduleTime
	}

	task.Status = "enable"

	// 启动任务（需持有锁）
	m.mu.Lock()
	m.startTask(task)
	m.tasks[task.Id] = task
	m.mu.Unlock()

	// 保存到Redis
	data, marshalErr := json.Marshal(task)
	if marshalErr != nil {
		return fmt.Errorf("failed to marshal task: %v", marshalErr)
	}
	if err := m.rdb.HSet(ctx, m.cronKey, task.Id, data).Err(); err != nil {
		return err
	}

	return nil
}

// RemoveTask 移除定时任务
func (m *CronManager) RemoveTask(ctx context.Context, taskId string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskId]
	if !ok {
		return fmt.Errorf("task not found: %s", taskId)
	}

	// 停止任务
	if task.EntryId > 0 {
		m.scheduler.RemoveCronTask(task.EntryId)
	}
	if task.timer != nil {
		task.timer.Stop()
	}

	// 从Redis删除
	if err := m.rdb.HDel(ctx, m.cronKey, taskId).Err(); err != nil {
		return err
	}

	delete(m.tasks, taskId)
	return nil
}

// EnableTask 启用定时任务
func (m *CronManager) EnableTask(ctx context.Context, taskId string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskId]
	if !ok {
		return fmt.Errorf("task not found: %s", taskId)
	}

	if task.Status == "enable" {
		return nil
	}

	task.Status = "enable"
	m.startTask(task)

	// 更新Redis
	data, marshalErr := json.Marshal(task)
	if marshalErr != nil {
		logx.Errorf("[CronManager] Failed to marshal task for redis update: taskId=%s, err=%v", taskId, marshalErr)
		return nil
	}
	return m.rdb.HSet(ctx, m.cronKey, taskId, data).Err()
}

// DisableTask 禁用定时任务
func (m *CronManager) DisableTask(ctx context.Context, taskId string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskId]
	if !ok {
		return fmt.Errorf("task not found: %s", taskId)
	}

	if task.Status == "disable" {
		return nil
	}

	// 停止任务
	if task.EntryId > 0 {
		m.scheduler.RemoveCronTask(task.EntryId)
		task.EntryId = 0
	}
	if task.timer != nil {
		task.timer.Stop()
		task.timer = nil
	}

	task.Status = "disable"

	// 保留Redis缓存但更新状态为disable（避免executeTask回退到MongoDB时读到过时数据）
	data, marshalErr := json.Marshal(task)
	if marshalErr != nil {
		logx.Errorf("[CronManager] Failed to marshal task for redis update on disable: taskId=%s, err=%v", taskId, marshalErr)
		return nil
	}
	if err := m.rdb.HSet(ctx, m.cronKey, taskId, data).Err(); err != nil {
		logx.Errorf("[CronManager] Failed to update task status in redis on disable: taskId=%s, err=%v", taskId, err)
	}

	return nil
}

// GetTasks 获取所有定时任务（并发安全，返回快照切片）
func (m *CronManager) GetTasks() []*CronTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tasks := make([]*CronTask, 0, len(m.tasks))
	for _, task := range m.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// GetTask 根据 taskId 获取单个定时任务（并发安全）
func (m *CronManager) GetTask(taskId string) (*CronTask, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	task, ok := m.tasks[taskId]
	return task, ok
}

// startTask 启动定时任务
func (m *CronManager) startTask(task *CronTask) {
	if task.ScheduleType == "once" {
		// 指定时间执行一次
		scheduleTime, err := time.ParseInLocation("2006-01-02 15:04:05", task.ScheduleTime, time.Local)
		if err != nil {
			return
		}

		// 如果时间已过，不启动
		if scheduleTime.Before(time.Now()) {
			return
		}

		// 使用定时器在指定时间执行
		duration := time.Until(scheduleTime)
		task.timer = time.NewTimer(duration)
		// 记录启动时的任务对象指针，用于检测 ReloadTask 是否替换了任务
		originalTask := task
		go func(t *CronTask) {
			<-t.timer.C
			m.mu.Lock()
			currentTask, ok := m.tasks[t.Id]
			enabled := ok && currentTask.Status == "enable"
			// 清理已触发的 timer 引用
			// 只有当内存中的任务对象仍然是启动时的对象时，才清理 timer
			// 如果 ReloadTask 已替换任务对象，新对象有自己的 timer，不需要清理
			if ok && currentTask == originalTask {
				currentTask.timer = nil
			}
			m.mu.Unlock()
			if enabled {
				m.executeTask(currentTask)
			}
		}(task)
	} else {
		// Cron表达式
		if task.CronSpec == "" {
			return
		}
		entryId, err := m.scheduler.AddCronTask(task.CronSpec, func() {
			// 从 map 中获取最新的 task 指针，避免闭包捕获旧对象
			m.mu.Lock()
			currentTask, ok := m.tasks[task.Id]
			m.mu.Unlock()
			if ok && currentTask.Status == "enable" {
				m.executeTask(currentTask)
			}
		})
		if err != nil {
			return
		}
		task.EntryId = entryId
	}
}

// executeTask 执行定时任务
func (m *CronManager) executeTask(task *CronTask) {
	ctx := context.Background()

	// === 阶段1: 加锁仅获取任务ID，快速释放锁 ===
	m.mu.Lock()
	taskId := task.Id
	m.mu.Unlock()

	// === 阶段2: 无锁状态下做IO操作（读取最新配置） ===
	var latestTarget, latestConfig, latestName, latestWorkspaceId, latestMainTaskId, latestTaskName string
	var latestScheduleType, latestCronSpec, latestScheduleTime string
	configLoaded := false

	// 从Redis重新读取最新配置
	latestData, err := m.rdb.HGet(ctx, m.cronKey, taskId).Result()
	if err == nil {
		var latestTask CronTask
		if json.Unmarshal([]byte(latestData), &latestTask) == nil {
			latestTarget = latestTask.Target
			latestConfig = latestTask.Config
			latestName = latestTask.Name
			latestWorkspaceId = latestTask.WorkspaceId
			latestMainTaskId = latestTask.MainTaskId
			latestTaskName = latestTask.TaskName
			latestScheduleType = latestTask.ScheduleType
			latestCronSpec = latestTask.CronSpec
			latestScheduleTime = latestTask.ScheduleTime
			configLoaded = true
		}
	} else {
		// Redis未命中，从MongoDB回退读取
		if m.taskSrc != nil {
			logx.Infof("[CronManager] Redis miss for task %s, falling back to MongoDB", taskId)
			td, mongoErr := m.taskSrc.FindCronTaskByCronTaskId(ctx, taskId)
			if mongoErr == nil && td != nil {
				latestTarget = td.Target
				latestConfig = td.Config
				latestName = td.Name
				latestWorkspaceId = td.WorkspaceId
				latestMainTaskId = td.MainTaskId
				latestTaskName = td.TaskName
				latestScheduleType = td.ScheduleType
				latestCronSpec = td.CronSpec
				latestScheduleTime = td.ScheduleTime
				configLoaded = true
				// 回写Redis缓存（无锁IO），包含完整字段
				cacheTask := &CronTask{
					Id: taskId, Name: latestName, ScheduleType: latestScheduleType,
					CronSpec: latestCronSpec, ScheduleTime: latestScheduleTime,
					WorkspaceId: latestWorkspaceId, MainTaskId: latestMainTaskId,
					TaskName: latestTaskName, Target: latestTarget, Config: latestConfig,
					Status: td.Status, LastRunTime: td.LastRunTime, NextRunTime: td.NextRunTime,
				}
				if data, marshalErr := json.Marshal(cacheTask); marshalErr != nil {
					logx.Errorf("[CronManager] Failed to marshal task for redis writeback: %v", marshalErr)
				} else {
					m.rdb.HSet(ctx, m.cronKey, taskId, data)
				}
			} else if mongoErr != nil {
				logx.Errorf("[CronManager] MongoDB fallback also failed for task %s: %v", taskId, mongoErr)
			}
		}
	}

	// === 阶段3: 加锁更新内存中的任务状态 ===
	m.mu.Lock()
	currentTask, ok := m.tasks[taskId]
	if !ok {
		// 任务已被删除，跳过
		m.mu.Unlock()
		return
	}

	// 用最新配置覆盖，保留内存中的调度状态字段
	if configLoaded {
		currentTask.Target = latestTarget
		currentTask.Config = latestConfig
		currentTask.Name = latestName
		currentTask.WorkspaceId = latestWorkspaceId
		currentTask.MainTaskId = latestMainTaskId
		currentTask.TaskName = latestTaskName
		currentTask.ScheduleType = latestScheduleType
		currentTask.CronSpec = latestCronSpec
		currentTask.ScheduleTime = latestScheduleTime
	}

	// 更新最后执行时间及状态
	currentTask.LastRunTime = time.Now().Local().Format("2006-01-02 15:04:05")

	// 计算下次执行时间
	switch currentTask.ScheduleType {
	case "cron":
		// 修复 C-08：原忽略 Parse 错误，spec 非法时 schedule 为 nil，
		// schedule.Next() 触发 nil pointer panic 导致 executeTask 崩溃
		schedule, err := cronParser.Parse(currentTask.CronSpec)
		if err != nil || schedule == nil {
			logx.Errorf("[CronManager] Invalid cron spec for task %s: spec=%q, err=%v",
				taskId, currentTask.CronSpec, err)
			currentTask.NextRunTime = ""
		} else {
			currentTask.NextRunTime = schedule.Next(time.Now()).Local().Format("2006-01-02 15:04:05")
		}
	case "once":
		// 一次性任务执行后禁用
		currentTask.Status = "disable"
		currentTask.NextRunTime = ""
	}

	// 提取需要的数据用于后续IO
	execData := struct {
		cronTaskId, workspaceId, mainTaskId, taskName, target, config string
		lastRunTime, nextRunTime, status                              string
	}{
		cronTaskId:  currentTask.Id,
		workspaceId: currentTask.WorkspaceId,
		mainTaskId:  currentTask.MainTaskId,
		taskName:    currentTask.Name,
		target:      currentTask.Target,
		config:      currentTask.Config,
		lastRunTime: currentTask.LastRunTime,
		nextRunTime: currentTask.NextRunTime,
		status:      currentTask.Status,
	}

	// 序列化当前任务状态（锁内完成，保证一致性）
	taskData, marshalErr := json.Marshal(currentTask)
	m.mu.Unlock()

	if marshalErr != nil {
		logx.Errorf("[CronManager] Failed to marshal task for redis update: cronTaskId=%s, err=%v", taskId, marshalErr)
	} else {
		m.rdb.HSet(ctx, m.cronKey, taskId, taskData)
	}

	// === 阶段4: 无锁状态下做IO操作（写Redis计数器、更新MongoDB、发布消息） ===

	// 增加运行次数
	runCountKey := fmt.Sprintf("cscan:cron:runcount:%s", taskId)
	m.rdb.Incr(ctx, runCountKey)

	// 同步更新MongoDB中的运行时间信息
	if m.taskSrc != nil {
		if err := m.taskSrc.UpdateCronTaskRunInfo(ctx, taskId, execData.lastRunTime, execData.nextRunTime, execData.status); err != nil {
			logx.Errorf("[CronManager] Failed to update run info in MongoDB: cronTaskId=%s, err=%v", taskId, err)
		}
	}

	// 发布消息通知 API 服务创建新任务
	cronExecData, _ := json.Marshal(map[string]interface{}{
		"cronTaskId":  execData.cronTaskId,
		"workspaceId": execData.workspaceId,
		"mainTaskId":  execData.mainTaskId,
		"taskName":    execData.taskName,
		"target":      execData.target,
		"config":      execData.config,
	})
	m.rdb.Publish(ctx, "cscan:cron:execute", string(cronExecData))
}

// ReloadTask 重新加载单个任务
func (m *CronManager) ReloadTask(ctx context.Context, taskId string) error {
	// 阶段1: 停止现有任务（需加锁）
	m.mu.Lock()
	if existingTask, ok := m.tasks[taskId]; ok {
		if existingTask.EntryId > 0 {
			m.scheduler.RemoveCronTask(existingTask.EntryId)
			existingTask.EntryId = 0
		}
		if existingTask.timer != nil {
			existingTask.timer.Stop()
			existingTask.timer = nil
		}
	}
	m.mu.Unlock()

	// 阶段2: 无锁状态下从数据源获取最新任务数据（IO操作）
	var task *CronTask
	if m.taskSrc != nil {
		td, err := m.taskSrc.FindCronTaskByCronTaskId(ctx, taskId)
		if err == nil && td != nil {
			task = &CronTask{
				Id:           td.CronTaskId,
				Name:         td.Name,
				ScheduleType: td.ScheduleType,
				CronSpec:     td.CronSpec,
				ScheduleTime: td.ScheduleTime,
				WorkspaceId:  td.WorkspaceId,
				MainTaskId:   td.MainTaskId,
				TaskName:     td.TaskName,
				Target:       td.Target,
				Config:       td.Config,
				Status:       td.Status,
				LastRunTime:  td.LastRunTime,
				NextRunTime:  td.NextRunTime,
			}
		} else if err != nil {
			logx.Errorf("[CronManager] Failed to load task from MongoDB: cronTaskId=%s, err=%v", taskId, err)
		}
	}

	// 回退：从Redis获取任务数据
	if task == nil {
		taskData, err := m.rdb.HGet(ctx, m.cronKey, taskId).Result()
		if err != nil {
			m.mu.Lock()
			delete(m.tasks, taskId)
			m.mu.Unlock()
			return err
		}

		var t CronTask
		if err := json.Unmarshal([]byte(taskData), &t); err != nil {
			return err
		}
		t.Id = taskId
		task = &t
	}

	// 阶段3: 加锁更新内存中的任务并启动
	m.mu.Lock()
	// 如果启用则启动
	if task.Status == "enable" {
		m.startTask(task)
	}
	m.tasks[taskId] = task
	m.mu.Unlock()

	// 阶段4: 无锁状态下同步到Redis缓存
	if data, marshalErr := json.Marshal(task); marshalErr != nil {
		logx.Errorf("[CronManager] Failed to marshal task for redis sync: cronTaskId=%s, err=%v", taskId, marshalErr)
	} else {
		m.rdb.HSet(ctx, m.cronKey, taskId, data)
	}

	return nil
}

// RunTaskNow 立即执行任务
func (m *CronManager) RunTaskNow(ctx context.Context, taskId string) error {
	// 阶段1: 无锁状态下从Redis获取最新任务配置（IO操作）
	var latestTask CronTask
	latestData, err := m.rdb.HGet(ctx, m.cronKey, taskId).Result()
	if err != nil {
		// Redis未命中，从MongoDB回退读取
		if m.taskSrc != nil {
			logx.Infof("[CronManager] Redis miss for RunTaskNow %s, falling back to MongoDB", taskId)
			td, mongoErr := m.taskSrc.FindCronTaskByCronTaskId(ctx, taskId)
			if mongoErr != nil || td == nil {
				return fmt.Errorf("task not found: %s", taskId)
			}
			latestTask = CronTask{
				Id: td.CronTaskId, Name: td.Name, ScheduleType: td.ScheduleType,
				CronSpec: td.CronSpec, ScheduleTime: td.ScheduleTime,
				WorkspaceId: td.WorkspaceId, MainTaskId: td.MainTaskId,
				TaskName: td.TaskName, Target: td.Target, Config: td.Config,
				Status: td.Status, LastRunTime: td.LastRunTime, NextRunTime: td.NextRunTime,
			}
		} else {
			return fmt.Errorf("task not found: %s", taskId)
		}
	} else {
		if err := json.Unmarshal([]byte(latestData), &latestTask); err != nil {
			return err
		}
	}

	// 阶段2: 加锁更新内存中的任务对象
	m.mu.Lock()
	// 获取或创建内存中的 task 对象
	task, ok := m.tasks[taskId]
	if !ok {
		task = &CronTask{Id: taskId}
		m.tasks[taskId] = task
	}

	// 用最新配置覆盖，保留运行时字段（EntryId, timer）
	task.Target = latestTask.Target
	task.Config = latestTask.Config
	task.Name = latestTask.Name
	task.WorkspaceId = latestTask.WorkspaceId
	task.MainTaskId = latestTask.MainTaskId
	task.TaskName = latestTask.TaskName
	task.ScheduleType = latestTask.ScheduleType
	task.CronSpec = latestTask.CronSpec
	task.ScheduleTime = latestTask.ScheduleTime
	task.Status = latestTask.Status
	task.LastRunTime = latestTask.LastRunTime
	task.NextRunTime = latestTask.NextRunTime
	m.mu.Unlock()

	// 阶段3: 无锁状态下执行任务
	go m.executeTask(task)
	return nil
}

// StartMessageSubscriber 启动消息订阅（含自动重连）
func (m *CronManager) StartMessageSubscriber(ctx context.Context) {
	go func() {
		retryDelay := 5 * time.Second
		maxRetryDelay := 60 * time.Second

		for {
			pubsub := m.rdb.Subscribe(ctx, "cscan:cron:reload", "cscan:cron:remove", "cscan:cron:runnow")

			ch := pubsub.Channel()
		subscribeLoop:
			for {
				select {
				case <-m.stopCh:
					pubsub.Close()
					return
				case msg, ok := <-ch:
					if !ok {
						break subscribeLoop
					}
					switch msg.Channel {
					case "cscan:cron:reload":
						m.ReloadTask(ctx, msg.Payload)
					case "cscan:cron:remove":
						m.mu.Lock()
						if task, ok := m.tasks[msg.Payload]; ok {
							if task.EntryId > 0 {
								m.scheduler.RemoveCronTask(task.EntryId)
							}
							if task.timer != nil {
								task.timer.Stop()
							}
							delete(m.tasks, msg.Payload)
						}
						m.mu.Unlock()
					case "cscan:cron:runnow":
						m.RunTaskNow(ctx, msg.Payload)
					}
				}
			}
			pubsub.Close()

			// 检查是否已停止
			select {
			case <-m.stopCh:
				return
			default:
			}

			logx.Errorf("[CronManager] Redis subscription disconnected, reconnecting in %v...", retryDelay)
			time.Sleep(retryDelay)
			// 指数退避，最大60秒
			if retryDelay < maxRetryDelay {
				retryDelay = retryDelay * 2
				if retryDelay > maxRetryDelay {
					retryDelay = maxRetryDelay
				}
			}
		}
	}()
}

// Stop 停止定时任务管理器，释放所有资源
func (m *CronManager) Stop() {
	logx.Info("[CronManager] Stopping cron manager...")

	// 通知订阅者退出
	close(m.stopCh)

	// 停止所有任务
	m.mu.Lock()
	for _, task := range m.tasks {
		if task.EntryId > 0 {
			m.scheduler.RemoveCronTask(task.EntryId)
		}
		if task.timer != nil {
			task.timer.Stop()
		}
	}
	m.mu.Unlock()

	logx.Info("[CronManager] Cron manager stopped")
}
