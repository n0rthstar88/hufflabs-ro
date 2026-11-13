-- Dumps a Gravity id table + name table pair (compiled Lua 5.1, run in an empty environment) as plain Lua.
-- usage: lua5.1 lubdump.lua <ids.lub> <names.lub> <outdir> [<id table> <name table> <ids out> <names out>]
-- defaults: jobtbl / JobNameTable -> npcidentity.lub / jobname.lub
local idtbl, nametbl = arg[4] or "jobtbl", arg[5] or "JobNameTable"
local idout, nameout = arg[6] or "npcidentity.lub", arg[7] or "jobname.lub"
local function sandboxload(path, env)
  local f = assert(loadfile(path)); setfenv(f, env); f(); return env
end
local env = sandboxload(arg[1], {})
local ids = assert(env[idtbl], "no " .. idtbl)
local env2 = {}; env2[idtbl] = ids
sandboxload(arg[2], env2)
local names = assert(env2[nametbl], "no " .. nametbl)
local keys = {}
for k in pairs(ids) do keys[#keys + 1] = k end
table.sort(keys, function(a, b) if ids[a] ~= ids[b] then return ids[a] < ids[b] end return a < b end)
local o = assert(io.open(arg[3] .. "/" .. idout, "w"))
o:write(idtbl, " = {\n")
for _, k in ipairs(keys) do o:write(string.format("\t%s = %d,\n", k, ids[k])) end
o:write("}\n"); o:close()
local byid = {}
for _, k in ipairs(keys) do byid[ids[k]] = byid[ids[k]] or k end
local nids = {}
for id in pairs(names) do nids[#nids + 1] = id end
table.sort(nids)
o = assert(io.open(arg[3] .. "/" .. nameout, "w"))
o:write(nametbl, " = {\n")
for _, id in ipairs(nids) do
  local key = byid[id] and (idtbl .. "." .. byid[id]) or tostring(id)
  o:write(string.format("\t[%s] = %q,\n", key, names[id]))
end
o:write("}\n"); o:close()
io.stderr:write(#keys, " ", idtbl, " entries, ", #nids, " ", nametbl, " entries\n")
