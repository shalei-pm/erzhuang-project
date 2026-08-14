# 门店空间资源查看 3.0 设计

日期：2026-08-13

## 背景

当前二壮项目的“门店空间资源”模块承担了门店、录像机、通道、设计图标注、AI 区域识别、人工确认和 H5 Monitor 入口等多种职责。随着公司业务库已经维护工控机、NVR、摄像头、诊室/床位和设备区域绑定关系，二壮继续维护一套独立空间映射会造成重复维护和数据口径分裂。

业务确认 3.0 不需要二壮继续维护其他区域分类，也不需要在本阶段改造监控播放链路。3.0 的目标是把二壮中的空间资源主流程改为“只读查看公司业务库映射结果”，用于检查门店空间资源配置是否完整。

## 产品定位

二壮 3.0「门店空间资源查看」是基于公司业务库的只读查看模块，用于展示已部署工控机门店的工控机、录像机、摄像头与业务空间三层结构的绑定关系。

模块只展示业务库事实，不在二壮侧维护空间、通道或映射关系。

## 目标

- 门店列表只展示已部署工控机的门店。
- 门店列表和详情数据来自公司业务库的只读查询结果。
- 门店详情展示业务库三层空间结构：
  - `level=1`：大类。
  - `level=2`：房间/区域。
  - `level=3`：床位/子区域。
- 门店详情展示空间与摄像头、NVR、工控机之间的绑定关系。
- 展示映射完整性和异常项，帮助业务确认配置是否完整。
- 保留现有登录、权限、门店范围权限和当前 H5 Monitor 监控查看方式。

## 非目标

- 不做设计图上传。
- 不做设计图标注。
- 不做 AI 通道识别。
- 不做批量识别区域。
- 不做人工确认通道归属。
- 不在二壮侧新增、编辑或删除门店、录像机、通道、空间或绑定关系。
- 不在 3.0 改造 H5 Monitor 播放页、萤石云取流、播放器兼容逻辑或工控机取流。
- 不继续使用二壮旧的“面诊室 / 治疗室 / 美容室”固定统计口径。
- 不保存验收状态、异常备注或人工补充分类。

## 业务库表关系

3.0 读取以下业务库表：

- `tb_crm_admin_tenant`：门店/租户。
- `tb_crm_iot_device`：物联网设备，包括工控机、NVR 和摄像头通道。
- `tb_crm_consulting_room`：业务空间，包括大类、房间/区域、床位/子区域。
- `tb_crm_iot_area_device_relation`：空间与设备绑定关系。

核心关系：

```text
tb_crm_admin_tenant.id
  = tenant_id
  = 二壮 external_org_id

tb_crm_consulting_room.tenant_id
  -> tb_crm_admin_tenant.id

tb_crm_iot_device.tenant_id
  -> tb_crm_admin_tenant.id

tb_crm_iot_area_device_relation.area_id
  -> tb_crm_consulting_room.id

tb_crm_iot_area_device_relation.device_id
  -> tb_crm_iot_device.id
  where tb_crm_iot_device.category = 'camera'

camera.parent_id
  -> tb_crm_iot_device.id
  where tb_crm_iot_device.category = 'nvr'
```

设备类型：

- `category='edge'`：工控机/边缘设备。
- `category='nvr'`：录像机。
- `category='camera'`：摄像头通道。

摄像头通道的 `hardware_id` 目前样例形态为：

```text
NVRCHANNEL:{nvr_device_id}-{channel_no}
```

例如：

```text
NVRCHANNEL:22-1
```

## 门店列表设计

模块名称：

- `门店空间资源查看`

列表只展示满足以下条件的门店：

- `tb_crm_admin_tenant.status = 1`。
- 存在 `tb_crm_iot_device.tenant_id = tb_crm_admin_tenant.id`。
- 该设备满足 `category='edge'`、`status=1`、`deleted_at is null`。

列表字段建议：

- 门店名称。
- 城市或城市 ID 映射后的城市名。
- 机构 ID / `tenant_id`。
- 工控机数。
- NVR 数。
- 摄像头数。
- 空间数。
- 已绑定摄像头数。
- 未绑定摄像头数。
- 离线设备数。
- 操作：查看详情、查看监控。

列表顶部统计建议：

- 共 N 家门店。
- 工控机 N。
- 录像机 N。
- 摄像头 N。
- 已绑定 N。
- 异常 N。

列表不再展示旧口径：

- 面诊室。
- 治疗室。
- 美容室。

## 门店详情设计

详情页标题：

```text
{门店名} / 空间资源映射
```

详情页只读展示，不出现新增、编辑、删除、扫描、识别、确认、保存等操作。

详情页建议分为三个视角：

### 空间视角

按业务库空间树展示：

```text
level=1 大类
  level=2 房间/区域
    level=3 床位/子区域
      绑定摄像头
```

每个空间节点展示：

- 空间 ID。
- 空间名称。
- `code`。
- `level`。
- `status`。
- 父级空间。
- 绑定摄像头数量。

每个绑定摄像头展示：

- `device_id`。
- 摄像头名称。
- `hardware_id`。
- 摄像头 IP。
- 摄像头状态。
- 摄像头在线状态。
- 所属 NVR。
- 通道号。

### 设备视角

按设备树展示：

```text
工控机 edge
  NVR
    摄像头 camera
      绑定空间路径
```

每个工控机展示：

- 设备 ID。
- `hardware_id`。
- 名称。
- IP。
- 心跳时间。
- 在线状态。
- `ext_params` 中的代理端口摘要。

每个 NVR 展示：

- 设备 ID。
- 名称。
- SN。
- IP。
- provider。
- 在线状态。
- 摄像头数量。

每个摄像头展示：

- 设备 ID。
- 摄像头名称。
- `hardware_id`。
- 解析出的通道号。
- IP。
- 在线状态。
- 绑定空间路径。

### 异常项

异常项只根据业务库事实即时计算，不保存到二壮。

建议第一版展示：

- 有摄像头但未绑定空间。
- 空间绑定了摄像头，但空间 `status != 1`。
- 绑定关系指向不存在或已删除的摄像头。
- 摄像头父级 NVR 不存在或已删除。
- 工控机离线。
- NVR 离线。
- 摄像头离线。
- 同一摄像头绑定多个空间。
- 同一空间绑定多个摄像头。

“同一空间绑定多个摄像头”不一定是错误，第一版可标记为提示级别，而不是阻断级别。

## 监控查看边界

3.0 不改 H5 Monitor。

当前怎么查看监控，3.0 仍然怎么查看：

- 不改变 H5 Monitor 路由。
- 不改变萤石云 API 取流。
- 不改变播放器组件。
- 不改变 Windows H.265 fallback 修复。
- 不改变普通查看用户的监控门店范围权限。

门店列表和详情里的“查看监控”入口可以沿用现有入口和权限逻辑。若业务库门店列表和现有 H5 Monitor 可播放门店暂时不完全一致，3.0 需要在 UI 上给出可读状态，例如“当前监控查看仍使用旧链路，未配置时入口不可用”。

工控机/NVR 取流能力后续作为 3.1 单独设计和实施。

## 旧能力处理

3.0 主流程隐藏或下线以下入口：

- 新增门店。
- 编辑门店。
- 删除门店。
- 添加录像机。
- 删除录像机。
- 扫描通道。
- 识别区域。
- 单通道识别。
- 通道人工确认。
- 设计图上传。
- 设计图标注。
- 保存设计图区域。

旧代码和数据不在 3.0 设计阶段直接删除。实施前需要先做代码和数据库使用面的备份/归档策略：

- 给当前 2.x 稳定状态打 Git tag 或保留备份分支。
- 确认公司 GitLab 固定分支有可回滚提交。
- 归档旧功能相关文档和入口说明。
- 确认 3.0 不再写旧通道识别、标注和确认数据。

## 权限设计

3.0 沿用现有登录和角色：

- 管理员。
- 编辑运维。
- 普通查看。

由于 3.0 主流程只读，管理员、编辑运维和普通查看在“门店空间资源查看”中的写能力差异可以收敛为无写操作。

普通查看用户的门店范围权限继续生效：

- 用户只能在授权门店看到或进入相关监控入口。
- 门店空间资源查看列表是否也按该门店范围过滤，需要在实施前确认。推荐第一版仍沿用当前门店范围权限，避免普通查看用户看到不负责门店的设备映射。

## 数据访问设计

3.0 后端建议新增只读 repository，而不是把业务库表混入旧二壮 store-space repository。

建议边界：

- `BusinessStoreRepository`：读取业务库门店、空间、设备、绑定关系。
- `StoreSpaceResourceViewService`：把业务库记录聚合为列表和详情响应。
- `StoreSpaceResourceViewHandler`：提供前端只读 API。

业务库连接通过运行时环境变量或 K8s Secret 注入，不写入仓库。

建议最小接口：

```text
GET /api/store-space-resource-view/stores
GET /api/store-space-resource-view/stores/{tenantId}
```

也可以沿用现有 `/api/store-space/stores` 路径，但需要避免旧写接口和新只读语义混在一起。推荐新增只读路径，前端页面切换到新 API。

## 数据模型输出建议

门店列表响应：

```text
tenant_id
store_name
hospital_name
city_id
edge_count
nvr_count
camera_count
space_count
bound_camera_count
unbound_camera_count
offline_device_count
warning_count
can_view_monitor
```

门店详情响应：

```text
tenant
edges[]
nvrs[]
cameras[]
spaces[]
relations[]
space_tree[]
device_tree[]
issues[]
```

每条 `issue` 建议包含：

```text
severity
type
message
entity_type
entity_id
```

## 待确认问题

实施前仍需确认：

1. `tb_crm_consulting_room.dict_id` 对应哪张字典表，是否需要在 3.0 展示字典名称。
2. `province_id` / `city_id` 是否有可读字典表，用于把城市 ID 转成城市名。
3. `tb_crm_iot_area_device_relation.function_type` 有哪些取值，哪些表示摄像头监控绑定。
4. `status` 和 `online_status` 的状态枚举是否只使用已知值：
   - 设备 `status`：1 启用，2 禁用。
   - 设备 `online_status`：1 在线，2 离线。
   - 空间 `status`：0 未启用，1 启用。
5. 工控机和 NVR 的关系是否只通过同一门店归属推导，还是有显式父子/代理关系。
6. 业务库只读账号在公司 K8s 环境的网络白名单、Secret 注入和最小权限。
7. 普通查看用户是否应在“门店空间资源查看”列表中只看到授权门店。推荐是。

## 分阶段建议

### 3.0-alpha

- 新增业务库只读 API。
- 门店列表展示有工控机门店。
- 门店详情展示空间树和设备树。
- 展示基础异常项。
- 隐藏旧维护入口。
- 不改 H5 Monitor。

### 3.0 正式

- 完成权限过滤。
- 完成城市/字典展示。
- 完成线上回归。
- 将模块名称、导航和页面文案统一改为“门店空间资源查看”。

### 3.1

- 单独研究工控机/NVR 取流或截图。
- 决定是否替代萤石云 API，或作为 fallback。

### 后续设计图版本

如果后续重新需要设计图，建议基于业务库空间树叠加设计图可视化，而不是恢复“设计图标注作为主数据”的旧模型。

## 验收标准

- 门店列表只展示部署了启用工控机的门店。
- 门店列表统计来自业务库，且不随分页变化。
- 门店详情可完整展示该门店业务库空间三层结构。
- 门店详情可展示空间与摄像头绑定关系。
- 门店详情可展示工控机、NVR、摄像头在线状态。
- 未绑定摄像头、离线设备、无效绑定能出现在异常项。
- 页面不出现旧写操作、识别、标注、确认入口。
- H5 Monitor 入口和播放行为与 2.x 保持一致。
- 普通查看用户门店范围权限不回退。

