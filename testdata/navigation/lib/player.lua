function make_player()
  local p = {x = 0}
  function p:update()
    self.x += 1
  end
  return p
end
