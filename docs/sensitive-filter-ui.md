# 敏感词过滤设置界面

本文档提供了敏感词过滤设置界面的前端代码示例，可以根据项目的前端框架进行适当调整。

## 敏感词配置文件

敏感词列表和默认响应现在存储在配置文件中：

1. 敏感词列表文件：`/data/config/sensitive_words.txt`
2. 敏感词响应文件：`/data/config/sensitive_response.txt`

这些文件路径可以通过环境变量进行配置：
- `SENSITIVE_WORDS_FILE`：敏感词列表文件路径
- `SENSITIVE_RESPONSE_FILE`：敏感词响应文件路径

系统会在启动时自动加载这些文件。如果文件不存在，系统会创建默认文件。

## 敏感词格式说明

敏感词列表支持两种格式：
1. 每行一个敏感词
2. 多个敏感词用英文逗号(,)分隔

系统会自动解析这两种格式，您可以根据需要选择使用哪种格式。

## React 组件示例

```jsx
import React, { useState, useEffect } from 'react';
import { API, showSuccess, showError } from '../helpers';

const SensitiveFilterSettings = () => {
  const [enabled, setEnabled] = useState(false);
  const [sensitiveWords, setSensitiveWords] = useState('');
  const [response, setResponse] = useState('');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    // 获取当前设置
    const fetchSettings = async () => {
      try {
        const res = await API.get('/api/options');
        if (res.data.success) {
          const options = res.data.data;
          setEnabled(options.SensitiveFilterEnabled === 'true');
          setSensitiveWords(options.SensitiveWords || '');
          setResponse(options.SensitiveFilterResponse || '您的请求包含敏感内容，已被系统拦截。');
        }
      } catch (error) {
        showError('获取设置失败');
        console.error(error);
      }
    };
    fetchSettings();
  }, []);

  const updateSetting = async (key, value) => {
    setLoading(true);
    try {
      const res = await API.put('/api/setting/sensitive-filter', {
        key,
        value
      });
      if (res.data.success) {
        showSuccess('设置已更新');
      } else {
        showError(res.data.message || '更新失败');
      }
    } catch (error) {
      showError('更新设置失败');
      console.error(error);
    } finally {
      setLoading(false);
    }
  };

  const handleEnableChange = (e) => {
    const newValue = e.target.checked;
    setEnabled(newValue);
    updateSetting('SensitiveFilterEnabled', newValue.toString());
  };

  const handleWordsChange = (e) => {
    setSensitiveWords(e.target.value);
  };

  const handleResponseChange = (e) => {
    setResponse(e.target.value);
  };

  const handleWordsSubmit = () => {
    updateSetting('SensitiveWords', sensitiveWords);
  };

  const handleResponseSubmit = () => {
    updateSetting('SensitiveFilterResponse', response);
  };

  return (
    <div className="sensitive-filter-settings">
      <h2>敏感词过滤设置</h2>
      
      <div className="setting-item">
        <h3>启用敏感词过滤</h3>
        <div className="form-check form-switch">
          <input
            className="form-check-input"
            type="checkbox"
            id="sensitiveFilterEnabled"
            checked={enabled}
            onChange={handleEnableChange}
            disabled={loading}
          />
          <label className="form-check-label" htmlFor="sensitiveFilterEnabled">
            {enabled ? '已启用' : '已禁用'}
          </label>
        </div>
        <p className="text-muted">启用后，系统将对大模型的输入和输出进行敏感词检测</p>
      </div>
      
      <div className="setting-item">
        <h3>敏感词列表</h3>
        <div className="form-group">
          <textarea
            className="form-control"
            rows="5"
            value={sensitiveWords}
            onChange={handleWordsChange}
            placeholder="请输入敏感词，每行一个敏感词或用英文逗号分隔"
            disabled={loading || !enabled}
          />
          <small className="form-text text-muted">支持两种格式：1. 每行一个敏感词；2. 多个敏感词用英文逗号(,)分隔。设置后会自动保存到配置文件中。</small>
        </div>
        <button
          className="btn btn-primary mt-2"
          onClick={handleWordsSubmit}
          disabled={loading || !enabled}
        >
          {loading ? '保存中...' : '保存敏感词列表'}
        </button>
      </div>
      
      <div className="setting-item">
        <h3>检测到敏感词时的响应内容</h3>
        <div className="form-group">
          <textarea
            className="form-control"
            rows="3"
            value={response}
            onChange={handleResponseChange}
            placeholder="请输入检测到敏感词时的响应内容"
            disabled={loading || !enabled}
          />
          <small className="form-text text-muted">当检测到敏感词时，将返回此内容给用户。设置后会自动保存到配置文件中。</small>
        </div>
        <button
          className="btn btn-primary mt-2"
          onClick={handleResponseSubmit}
          disabled={loading || !enabled}
        >
          {loading ? '保存中...' : '保存响应内容'}
        </button>
      </div>
    </div>
  );
};

export default SensitiveFilterSettings;
```

## 集成到现有系统

1. 将上述组件添加到系统的设置页面中
2. 在路由配置中添加对应的路由
3. 在菜单中添加对应的入口

## API 接口说明

### 获取设置

- 请求方式：GET
- 请求路径：/api/options
- 响应示例：
```json
{
  "success": true,
  "data": {
    "SensitiveFilterEnabled": "true",
    "SensitiveWords": "敏感词1\n敏感词2\n敏感词3",
    "SensitiveFilterResponse": "您的请求包含敏感内容，已被系统拦截。"
  }
}
```

### 更新设置

- 请求方式：PUT
- 请求路径：/api/setting/sensitive-filter
- 请求参数：
```json
{
  "key": "SensitiveFilterEnabled", // 或 "SensitiveWords" 或 "SensitiveFilterResponse"
  "value": "true" // 或敏感词列表字符串或响应内容
}
```
- 响应示例：
```json
{
  "success": true,
  "message": ""
}
```

## 配置文件说明

修改配置文件后，系统会自动重新加载敏感词列表和响应内容，无需重启服务。

### 敏感词列表文件示例 (/data/config/sensitive_words.txt)

```
敏感词1
敏感词2
敏感词3
```

### 敏感词响应文件示例 (/data/config/sensitive_response.txt)

```
您的请求包含敏感内容，已被系统拦截。
``` 