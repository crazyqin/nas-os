# 礼部：多系统管理Dashboard UI设计

## 概述
对标TrueNAS Dashboard widget化，设计nas-os多系统管理UI。

## UI组件设计

### 1. MultiSystemDashboard 主页面

```
┌─────────────────────────────────────────────────────────────────┐
│  🌐 NAS-OS Multi-System Manager        [全局搜索] [节点选择器]  │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────────┤
│  │ 📊 系统概览 (3节点在线)                                      │
│  │  总容量: 12TB | 总可用: 8TB | 健康节点: 3/3                 │
│  ├─────────────────────────────────────────────────────────────┤
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐                  │
│  │  │ Node A   │  │ Node B   │  │ Node C   │                  │
│  │  │ 98/100   │  │ 85/100   │  │ 92/100   │                  │
│  │  │ 🟢健康   │  │ 🟡警告   │  │ 🟢健康   │                  │
│  │  │ 4TB/6TB  │  │ 2TB/4TB  │  │ 2TB/2TB  │                  │
│  │  └──────────┘  └──────────┘  └──────────┘                  │
│  └─────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────────────────────────────────────────────────┤
│  │ 🔄 复制任务                                                 │
│  │  任务ID: task-001 | 状态: 进行中 | 进度: 45%                │
│  │  Node A → Node B | 速度: 50MB/s | 剩余: 2h                 │
│  ├─────────────────────────────────────────────────────────────┤
│  │ 📈 资源聚合                                                 │
│  │  CPU使用: 35% | 内存: 65% | 网络: 120MB/s                  │
│  └─────────────────────────────────────────────────────────────┤
└─────────────────────────────────────────────────────────────────┘
```

### 2. NodeCard Widget组件

```typescript
// NodeCard.tsx - 单节点状态卡片
interface NodeCardProps {
  node: NodeInfo;
  onSelect: (nodeId: string) => void;
  compact?: boolean;
}

interface NodeInfo {
  id: string;
  name: string;
  healthScore: number;
  status: 'healthy' | 'warning' | 'critical' | 'offline';
  storage: {
    total: number;
    used: number;
    available: number;
  };
  services: ServiceStatus[];
  alerts: Alert[];
}

// 组件结构
const NodeCard: React.FC<NodeCardProps> = ({ node }) => {
  return (
    <Card className="node-card">
      <Header>
        <NodeIcon status={node.status} />
        <Title>{node.name}</Title>
        <HealthBadge score={node.healthScore} />
      </Header>
      <Body>
        <StorageMeter used={node.storage.used} total={node.storage.total} />
        <ServiceList services={node.services} />
        {node.alerts.length > 0 && <AlertBadge count={node.alerts.length} />}
      </Body>
      <Footer>
        <QuickActions node={node} />
      </Footer>
    </Card>
  );
};
```

### 3. GlobalSearchBar 全局搜索

```typescript
// GlobalSearchBar.tsx - 全局搜索组件
interface SearchBarProps {
  onSearch: (query: SearchQuery) => void;
  scope?: 'current' | 'all';
}

const GlobalSearchBar: React.FC<SearchBarProps> = ({ onSearch, scope }) => {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(false);

  // 搜索类型快捷键
  const searchTypes = [
    { key: 'f', label: '文件', icon: '📁' },
    { key: 'v', label: '卷', icon: '💾' },
    { key: 's', label: '共享', icon: '🔗' },
    { key: 'u', label: '用户', icon: '👤' },
    { key: 'n', label: '节点', icon: '🌐' },
  ];

  return (
    <SearchContainer>
      <SearchInput
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder="搜索文件、设置、用户... (Ctrl+K)"
        onKeyDown={handleKeyDown}
      />
      <TypeSelector types={searchTypes} />
      <ScopeSelector scope={scope} />
      {results.length > 0 && <SearchResults results={results} />}
    </SearchContainer>
  );
};

// 搜索结果面板
const SearchResults: React.FC<{ results: SearchResult[] }> = ({ results }) => {
  return (
    <ResultsPanel>
      {results.map((result) => (
        <ResultItem key={result.id}>
          <ResultIcon type={result.type} />
          <ResultTitle>{result.name}</ResultTitle>
          <ResultPath>{result.path}</ResultPath>
          <NodeBadge nodeId={result.nodeId} />
        </ResultItem>
      ))}
    </ResultsPanel>
  );
};
```

### 4. NodeSwitcher 节点切换器

```typescript
// NodeSwitcher.tsx - 节点快速切换
interface NodeSwitcherProps {
  nodes: Node[];
  currentNode: string;
  onSelect: (nodeId: string) => void;
}

const NodeSwitcher: React.FC<NodeSwitcherProps> = ({ nodes, currentNode, onSelect }) => {
  return (
    <Dropdown>
      <Trigger>
        <CurrentNodeIcon />
        <CurrentNodeName />
        <ChevronIcon />
      </Trigger>
      <Menu>
        {nodes.map((node) => (
          <MenuItem key={node.id} onClick={() => onSelect(node.id)}>
            <NodeStatusIndicator status={node.status} />
            <NodeName>{node.name}</NodeName>
            <NodeVersion>v{node.version}</NodeVersion>
          </MenuItem>
        ))}
        <Divider />
        <AddNodeButton>添加新节点</AddNodeButton>
      </Menu>
    </Dropdown>
  );
};
```

### 5. FleetTaskProgress 复制任务进度

```typescript
// FleetTaskProgress.tsx - 复制任务进度展示
interface TaskProgressProps {
  task: ReplicationTask;
}

const FleetTaskProgress: React.FC<TaskProgressProps> = ({ task }) => {
  return (
    <ProgressCard>
      <Header>
        <TaskIcon />
        <TaskId>{task.id}</TaskId>
        <StatusBadge status={task.status} />
      </Header>
      <ProgressBar value={task.percentage} animated />
      <Details>
        <SourceNode>{task.sourceNode}</SourceNode>
        <ArrowIcon />
        <TargetNodes>{task.targetNodes.join(', ')}</TargetNodes>
      </Details>
      <Stats>
        <Speed>{task.speed} MB/s</Speed>
        <ETA>剩余 {formatETA(task.estimatedTime)}</ETA>
        <Transferred>{formatBytes(task.transferredBytes)}</Transferred>
      </Stats>
      <Actions>
        <PauseButton />
        <ResumeButton />
        <CancelButton />
      </Actions>
    </ProgressCard>
  );
};
```

## 响应式设计

| 屏幕尺寸 | 布局调整 |
|----------|----------|
| Desktop (>1200px) | 3列NodeCard + 侧边栏详情 |
| Tablet (768-1200px) | 2列NodeCard + 折叠详情 |
| Mobile (<768px) | 单列NodeCard + 全屏详情 |

## 交互设计

| 操作 | 热键 | 描述 |
|------|------|------|
| 全局搜索 | Ctrl+K | 打开搜索面板 |
| 切换节点 | Ctrl+N | 节点切换下拉 |
| 刷新状态 | Ctrl+R | 刷新所有节点状态 |
| 快速命令 | Ctrl+; | 打开命令面板 |

## 实现计划

| 阶段 | 任务 | 时间 |
|------|------|------|
| M1 | GlobalSearchBar组件 | 04-03 |
| M2 | NodeCard + NodeSwitcher | 04-05 |
| M3 | MultiSystemDashboard布局 | 04-08 |
| M4 | FleetTaskProgress | 04-10 |
| M5 | 响应式适配 | 04-15 |