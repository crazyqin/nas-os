// Package scrubsched 提供智能Scrub调度功能
package scrubsched

import (
	"log"
	"time"
)

// ========== 调度引擎 ==========

// Scheduler Scrub调度引擎，负责避峰调度算法.
type Scheduler struct {
	manager *Manager
}

// NewScheduler 创建调度引擎.
func NewScheduler(mgr *Manager) *Scheduler {
	return &Scheduler{
		manager: mgr,
	}
}

// Start 启动调度引擎主循环.
func (s *Scheduler) Start() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	log.Println("[scrubsched] 调度引擎已启动")

	for {
		select {
		case <-s.manager.stopCh:
			log.Println("[scrubsched] 调度引擎已停止")
			return
		case now := <-ticker.C:
			s.tick(now)
		}
	}
}

// tick 每次调度检查.
func (s *Scheduler) tick(now time.Time) {
	s.manager.mu.RLock()
	policies := make([]*Policy, 0, len(s.manager.policies))
	for _, p := range s.manager.policies {
		if p.Enabled {
			policyCopy := *p
			policies = append(policies, &policyCopy)
		}
	}
	s.manager.mu.RUnlock()

	for _, p := range policies {
		if s.shouldRun(p, now) {
			go s.executePolicy(p)
		}
	}

	// 检查避峰暂停/恢复
	s.checkPeakAvoidance(now)
}

// shouldRun 判断策略是否应该执行.
func (s *Scheduler) shouldRun(p *Policy, now time.Time) bool {
	if p.NextRun == nil {
		return false
	}

	// 检查是否到了执行时间
	if now.Before(*p.NextRun) {
		return false
	}

	// 检查是否已经在运行
	s.manager.mu.RLock()
	status, running := s.manager.statuses[p.PoolID]
	s.manager.mu.RUnlock()

	if running && (status.State == StateRunning || status.State == StatePaused) {
		return false
	}

	// 如果启用了避峰，检查是否在高峰窗口
	if s.manager.isInPeakWindow(p, now) {
		log.Printf("[scrubsched] 策略 %s 处于高峰窗口，推迟执行", p.Name)
		// 推迟到高峰窗口结束
		s.deferExecution(p, now)
		return false
	}

	return true
}

// executePolicy 执行策略对应的Scrub.
func (s *Scheduler) executePolicy(p *Policy) {
	log.Printf("[scrubsched] 执行策略: %s (池: %s)", p.Name, p.PoolID)

	s.manager.mu.Lock()
	// 创建运行状态
	now := time.Now()
	s.manager.statuses[p.PoolID] = &ScrubStatus{
		PoolID:    p.PoolID,
		State:     StateRunning,
		StartTime: &now,
		PolicyID:  p.ID,
		IsManual:  false,
	}

	// 更新策略执行时间
	p.LastRun = &now
	p.RunCount++
	nextRun := s.manager.calculateNextRun(p)
	p.NextRun = &nextRun
	s.manager.mu.Unlock()

	// 启动Scrub
	if s.manager.scrubExec != nil {
		if err := s.manager.scrubExec.StartScrub(p.PoolID); err != nil {
			log.Printf("[scrubsched] 启动Scrub失败: %v", err)
			s.manager.completeScrub(p.PoolID, StateFailed, 0, 0, err.Error())
			return
		}
	}

	// 启动进度监控
	go s.monitorProgress(p)
}

// monitorProgress 监控Scrub执行进度.
func (s *Scheduler) monitorProgress(p *Policy) {
	progressTicker := time.NewTicker(30 * time.Second)
	defer progressTicker.Stop()

	timeout := time.After(p.MaxDuration)

	for {
		select {
		case <-s.manager.stopCh:
			return
		case <-timeout:
			// 超时强制完成
			log.Printf("[scrubsched] 策略 %s 执行超时", p.Name)
			s.manager.completeScrub(p.PoolID, StateFailed, 0, 0, "超过最大执行时间限制")
			return
		case <-progressTicker.C:
			// 检查进度
			if s.manager.scrubExec != nil {
				progress, err := s.manager.scrubExec.GetScrubProgress(p.PoolID)
				if err != nil {
					log.Printf("[scrubsched] 获取进度失败: %v", err)
					continue
				}

				s.manager.mu.Lock()
				if status, ok := s.manager.statuses[p.PoolID]; ok {
					status.Progress = progress
					if status.StartTime != nil {
						status.ElapsedTime = int64(time.Since(*status.StartTime).Seconds())
					}
				}
				s.manager.mu.Unlock()

				// 检查是否完成
				if progress >= 100 {
					log.Printf("[scrubsched] 策略 %s 执行完成", p.Name)
					s.manager.completeScrub(p.PoolID, StateCompleted, 0, 0, "")
					return
				}

				// 检查是否仍在运行
				running, err := s.manager.scrubExec.IsScrubRunning(p.PoolID)
				if err != nil || !running {
					if err != nil {
						s.manager.completeScrub(p.PoolID, StateFailed, 0, 0, err.Error())
					} else {
						s.manager.completeScrub(p.PoolID, StateCompleted, 0, 0, "")
					}
					return
				}
			}
		}
	}
}

// checkPeakAvoidance 检查避峰暂停/恢复.
func (s *Scheduler) checkPeakAvoidance(now time.Time) {
	s.manager.mu.RLock()
	statuses := make(map[string]*ScrubStatus)
	for k, v := range s.manager.statuses {
		statusCopy := *v
		statuses[k] = &statusCopy
	}
	policies := make(map[string]*Policy)
	for k, v := range s.manager.policies {
		policyCopy := *v
		policies[k] = &policyCopy
	}
	s.manager.mu.RUnlock()

	for poolID, status := range statuses {
		if status.State != StateRunning && status.State != StatePaused {
			continue
		}

		// 查找关联的策略
		p, ok := policies[status.PolicyID]
		if !ok {
			continue
		}

		inPeak := s.manager.isInPeakWindow(p, now)

		if inPeak && status.State == StateRunning {
			// 在高峰窗口且正在运行 -> 暂停
			log.Printf("[scrubsched] 池 %s 进入高峰窗口，暂停Scrub", poolID)
			_ = s.manager.PauseScrub(poolID, "避峰调度：进入业务高峰时段")
		} else if !inPeak && status.State == StatePaused && status.PauseReason == "避峰调度：进入业务高峰时段" {
			// 不在高峰窗口且因避峰暂停 -> 恢复
			log.Printf("[scrubsched] 池 %s 离开高峰窗口，恢复Scrub", poolID)
			_ = s.manager.ResumeScrub(poolID)
		}
	}
}

// deferExecution 推迟策略执行到高峰窗口结束后.
func (s *Scheduler) deferExecution(p *Policy, now time.Time) {
	// 找到最近的高峰窗口结束时间
	weekday := int(now.Weekday())
	currentMinutes := now.Hour()*60 + now.Minute()

	earliestEnd := 24 * 60 // 最大值

	for _, w := range p.PeakWindows {
		if w.DayOfWeek != -1 && w.DayOfWeek != weekday {
			continue
		}
		endMinutes := w.EndHour*60 + w.EndMin
		startMinutes := w.StartHour*60 + w.StartMin

		if startMinutes <= endMinutes {
			if currentMinutes >= startMinutes && currentMinutes < endMinutes {
				if endMinutes < earliestEnd {
					earliestEnd = endMinutes
				}
			}
		} else {
			// 跨午夜
			if currentMinutes >= startMinutes {
				earliestEnd = 24*60 + endMinutes // 明天
			}
		}
	}

	if earliestEnd < 24*60*2 {
		// 计算推迟时间
		deferMinutes := earliestEnd - currentMinutes
		if deferMinutes <= 0 {
			deferMinutes += 24 * 60
		}
		nextRun := now.Add(time.Duration(deferMinutes) * time.Minute)

		s.manager.mu.Lock()
		if policy, ok := s.manager.policies[p.ID]; ok {
			policy.NextRun = &nextRun
		}
		s.manager.mu.Unlock()

		log.Printf("[scrubsched] 策略 %s 推迟到 %s 执行", p.Name, nextRun.Format("2006-01-02 15:04"))
	}
}
