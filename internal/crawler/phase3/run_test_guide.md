运行方式：                                                                                           
                                                                                                       
RIOT_API_KEY=<your_key> go test -tags e2e -v ./internal/crawler/phase3/ -run TestRun_realMatch       
                                                                                                       
执行后会输出 5 个步骤的日志，类似这样：                                                              
                                                                                                       
=== RUN   TestRun_realMatch                                                                          
    step 1: fetching match detail from Riot API                                                    
    fetched match KR_8169051579: queue=420 version=15.1.419 participants=10                          
    step 2: recording DTO to testdata/                                                               
    wrote testdata/KR_8169051579.json (48392 bytes)                                                  
    step 3: validating DTO                                                                           
    step 4: writing to database (test schema)                                                        
    step 5: validating database state                                                                
    inserted 10 participants
    inserted 10 perk rows                                                                            
    participants in DB:
    Lux                  10/2/5   enemy_missing_pings=3   time_played=1800
    ...                                                                                            
--- PASS
                                                                                                    
DTO 的完整 JSON 会保存到 testdata/KR_8169051579.json，可以直接翻阅 Riot API 返回了什么字段。         
   