local visits = 0
local uniques = 0

for _, key in pairs (KEYS) do
  local val = redis.pcall("GET", "hits_" .. key)
  if val then
    visits = visits + tonumber(val)
  end
end

local hll_keys = {}
for _, key in pairs (KEYS) do
  	table.insert(hll_keys, "hll_" .. key)
end
uniques = redis.pcall("PFCOUNT", unpack(hll_keys))

return {visits, uniques}
