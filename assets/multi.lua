local sum = 0
for _, key in pairs (KEYS) do
  local val = redis.pcall("GET", key)
  if val then
    sum = sum + tonumber(val)
  end
end
return sum
