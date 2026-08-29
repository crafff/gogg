# Azure VM 开机、关机与公网 IP 运维

本文适用于当前 GOGG Azure 环境：

| 项目 | 值 |
|---|---|
| 订阅 | Azure for Students |
| 资源组 | `rg-gogg-prod` |
| VM | `gogg-prod` |
| 区域 | `northcentralus` |
| VM 规格 | `Standard_D2s_v3` |
| 系统盘 | 64 GiB Standard SSD LRS |
| 当前公网 IP | `135.232.215.174`（删除前） |

所有 `az` 命令都在 Azure Portal 的 Cloud Shell（Bash）中执行。

## 1. 重要原则

- 必须让 VM 进入 `Stopped (deallocated)` / `VM deallocated`，才能停止 CPU 和内存的计算费用。
- 不要只在 Ubuntu 中运行 `sudo shutdown`；这可能只进入仍计费的 `Stopped (allocated)` 状态。
- `deallocate` 不会删除系统盘，PostgreSQL、Redis、Temporal、Docker 镜像、Asset 和生产配置都会保留。
- 不要删除 VM、系统盘、NIC、VNet 或资源组，否则可能丢失当前环境。
- 当前 Worker 没有创建，VM 重启后也不会自动运行 Worker。

## 2. 方案 A：保留静态公网 IP（推荐日常使用）

优点：命令最少，重新开机后地址仍是 `135.232.215.174`，项目配置不需要修改。

关机后预计保留费用：

- 64 GiB Standard SSD：约 `$4.80/月`
- Standard 静态公网 IPv4：约 `$3.65/月`
- 合计：约 `$8.45/月`

### 2.1 关机

```bash
az vm deallocate \
  --resource-group rg-gogg-prod \
  --name gogg-prod
```

确认状态：

```bash
az vm get-instance-view \
  --resource-group rg-gogg-prod \
  --name gogg-prod \
  --query "instanceView.statuses[?starts_with(code,'PowerState/')].displayStatus" \
  --output tsv
```

必须看到：

```text
VM deallocated
```

### 2.2 开机

```bash
az vm start \
  --resource-group rg-gogg-prod \
  --name gogg-prod
```

查看状态和 IP：

```bash
az vm show \
  --resource-group rg-gogg-prod \
  --name gogg-prod \
  --show-details \
  --query '{state:powerState,publicIp:publicIps}' \
  --output json
```

通常等待 2–4 分钟后访问：

```text
http://135.232.215.174
```

验证：

```bash
curl -fsS -o /dev/null -w 'status=%{http_code} time=%{time_total}s\n' \
  http://135.232.215.174/
```

## 3. 方案 B：关机时删除公网 IP（最低保留费用）

优点：关机期间省去约 `$3.65/月` 的 Standard 静态公网 IPv4 费用，只保留约 `$4.80/月` 的系统盘费用。

缺点：每次重新创建都会得到新的公网 IP，需要更新访问地址和 API issuer。不要依赖旧地址 `135.232.215.174`。

### 3.1 关机、解绑并删除公网 IP

先设置公共变量，并自动读取 NIC、IP configuration 和公网 IP 名称：

```bash
RESOURCE_GROUP="rg-gogg-prod"
VM_NAME="gogg-prod"

NIC_ID="$(az vm show \
  --resource-group "$RESOURCE_GROUP" \
  --name "$VM_NAME" \
  --query 'networkProfile.networkInterfaces[0].id' \
  --output tsv)"
NIC_NAME="${NIC_ID##*/}"

IP_CONFIG_NAME="$(az network nic ip-config list \
  --resource-group "$RESOURCE_GROUP" \
  --nic-name "$NIC_NAME" \
  --query '[0].name' \
  --output tsv)"

PUBLIC_IP_ID="$(az network nic ip-config show \
  --resource-group "$RESOURCE_GROUP" \
  --nic-name "$NIC_NAME" \
  --name "$IP_CONFIG_NAME" \
  --query 'publicIPAddress.id' \
  --output tsv)"
PUBLIC_IP_NAME="${PUBLIC_IP_ID##*/}"

printf 'NIC=%s\nIP_CONFIG=%s\nPUBLIC_IP=%s\n' \
  "$NIC_NAME" "$IP_CONFIG_NAME" "$PUBLIC_IP_NAME"
```

确认输出中的公网 IP 名称不是空值，然后解除分配 VM：

```bash
az vm deallocate \
  --resource-group "$RESOURCE_GROUP" \
  --name "$VM_NAME"
```

从 NIC 解绑公网 IP：

```bash
az network nic ip-config update \
  --resource-group "$RESOURCE_GROUP" \
  --nic-name "$NIC_NAME" \
  --name "$IP_CONFIG_NAME" \
  --remove publicIpAddress
```

确认已解绑；下面命令应输出 `null`：

```bash
az network nic ip-config show \
  --resource-group "$RESOURCE_GROUP" \
  --nic-name "$NIC_NAME" \
  --name "$IP_CONFIG_NAME" \
  --query 'publicIPAddress' \
  --output json
```

删除公网 IP 资源：

```bash
az network public-ip delete \
  --resource-group "$RESOURCE_GROUP" \
  --name "$PUBLIC_IP_NAME"
```

确认 VM 已解除分配，并且资源组中没有公网 IP：

```bash
az vm get-instance-view \
  --resource-group "$RESOURCE_GROUP" \
  --name "$VM_NAME" \
  --query "instanceView.statuses[?starts_with(code,'PowerState/')].displayStatus" \
  --output tsv

az network public-ip list \
  --resource-group "$RESOURCE_GROUP" \
  --output table
```

### 3.2 创建新公网 IP 并开机

重新设置变量，因为 Cloud Shell 会话可能已经结束：

```bash
RESOURCE_GROUP="rg-gogg-prod"
VM_NAME="gogg-prod"
LOCATION="northcentralus"
PUBLIC_IP_NAME="gogg-prodPublicIP"

NIC_ID="$(az vm show \
  --resource-group "$RESOURCE_GROUP" \
  --name "$VM_NAME" \
  --query 'networkProfile.networkInterfaces[0].id' \
  --output tsv)"
NIC_NAME="${NIC_ID##*/}"

IP_CONFIG_NAME="$(az network nic ip-config list \
  --resource-group "$RESOURCE_GROUP" \
  --nic-name "$NIC_NAME" \
  --query '[0].name' \
  --output tsv)"
```

创建新的 Standard 静态公网 IPv4：

```bash
az network public-ip create \
  --resource-group "$RESOURCE_GROUP" \
  --name "$PUBLIC_IP_NAME" \
  --location "$LOCATION" \
  --sku Standard \
  --allocation-method Static \
  --version IPv4
```

关联到现有 NIC：

```bash
az network nic ip-config update \
  --resource-group "$RESOURCE_GROUP" \
  --nic-name "$NIC_NAME" \
  --name "$IP_CONFIG_NAME" \
  --public-ip-address "$PUBLIC_IP_NAME"
```

读取新地址并启动 VM：

```bash
NEW_IP="$(az network public-ip show \
  --resource-group "$RESOURCE_GROUP" \
  --name "$PUBLIC_IP_NAME" \
  --query 'ipAddress' \
  --output tsv)"

echo "NEW_IP=$NEW_IP"

az vm start \
  --resource-group "$RESOURCE_GROUP" \
  --name "$VM_NAME"
```

等待 2–4 分钟，然后验证新地址：

```bash
curl -fsS -o /dev/null -w 'status=%{http_code} time=%{time_total}s\n' \
  "http://$NEW_IP/"
```

### 3.3 更新 GOGG 的 IP 配置

公网 IP 改变后，通过 SSH 更新 API issuer 并重建 API 容器：

```bash
ssh zrt@"$NEW_IP" \
  "sed -i -E 's|GOGG_AUTH_ISSUER: http://[0-9.]+|GOGG_AUTH_ISSUER: http://$NEW_IP|' /opt/gogg/docker-compose.prod.yml && \
   cd /opt/gogg && \
   docker compose -f docker-compose.prod.yml up -d --force-recreate api web"
```

验证：

```bash
curl -fsS -o /dev/null -w 'web=%{http_code} time=%{time_total}s\n' \
  "http://$NEW_IP/"

curl -fsS -o /dev/null -w 'asset=%{http_code} time=%{time_total}s\n' \
  "http://$NEW_IP/game-assets/16.13/manifest.json"
```

如果 SSH 报超时，通常是当前公网出口 IP 与 NSG 中允许的 SSH 来源不同。此时需要先在 Azure Portal 或 Cloud Shell 更新 `nsg-gogg-prod` 的 SSH 来源规则，不要把22端口永久开放给整个 Internet。

## 4. Docker 服务恢复行为

保留公网 IP 或删除公网 IP 都不会改变系统盘数据。VM 启动后，Docker 会根据 `restart: unless-stopped` 自动恢复：

- PostgreSQL
- Redis
- Temporal PostgreSQL
- Temporal
- API
- Web

Worker 当前没有创建，因此不会自动启动。

启动后可通过 SSH 检查：

```bash
ssh zrt@"${NEW_IP:-135.232.215.174}" \
  'cd /opt/gogg && docker compose -f docker-compose.prod.yml ps'
```

## 5. 如何选择

- 经常开关、希望操作简单：使用方案 A，保留静态公网 IP。
- 可能连续数周不开机、希望最低保留费用：使用方案 B，删除公网 IP。
- 后续绑定正式域名后，可以通过更新 DNS 记录隐藏 IP 变化，但 DNS、API issuer 和 HTTPS 配置仍需要同步更新。

## 6. 当前价格参考

按 North Central US 当前公开零售价估算：

- VM 运行计算费：约 `$0.10/小时`
- 64 GiB Standard SSD LRS：约 `$4.80/月`
- Standard 静态公网 IPv4：约 `$0.005/小时`，即约 `$3.65/月`
- 保留 IP 的关机成本：约 `$8.45/月`
- 删除 IP 的关机成本：约 `$4.80/月`

实际账单可能因学生额度、计费周期、税费、磁盘事务和网络出站流量略有差异。
