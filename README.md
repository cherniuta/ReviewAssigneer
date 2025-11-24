# ReviewAssigneer

### Запуск через Docker 

1. **Клонируйте репозиторий:**
```bash
git clone <repository-url>
cd ReviewAssigneer/ReviewAssigneer/api
```


2. **Запустите приложение одной командой**
```bash
docker-compose up 
```

3. **Проверьте работу приложения:**
```bash
curl -X GET "http://localhost:8080/users/getReview?user_id=u1"
```

##Остановка приложения
```bash
docker-compose down
```
